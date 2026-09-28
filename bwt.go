// Package bwtcodec 做 Burrows-Wheeler 变换与逆变换，只用标准库。
package bwtcodec

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	// ErrPrimaryOutOfRange 表示主索引不在 [0, n) 范围内。
	ErrPrimaryOutOfRange = errors.New("bwtcodec: 主索引越界")
	// ErrRLETruncated 表示游程数据在一个完整的 (字节, varint) 对之前结束。
	ErrRLETruncated = errors.New("bwtcodec: 游程数据截断")
	// ErrRLEVarintTooLong 表示 varint 超过 10 字节或无法用 uint64 表示。
	ErrRLEVarintTooLong = errors.New("bwtcodec: varint 超过 10 字节")
	// ErrRLEZeroRun 表示某个游程长度为 0。
	ErrRLEZeroRun = errors.New("bwtcodec: 游程长度为 0")
	// ErrRLELength 表示游程解码出的字节数与给定长度不一致。
	ErrRLELength = errors.New("bwtcodec: 游程解码长度不一致")
)

// Encode 返回 BWT 结果与主索引。主索引是排序旋转表里等于原串的那一行下标。
// 用后缀数组（倍增 + 计数排序）一次求出，O(n log n) 时间、O(n) 空间，
// 不为任何位置复制完整旋转。空输入返回空结果与主索引 0。
func Encode(data []byte) ([]byte, int) {
	n := len(data)
	if n == 0 {
		return nil, 0
	}
	sa := suffixArray(data)
	last := make([]byte, n)
	primary := 0
	for row, pos := range sa {
		if pos == 0 {
			last[row] = data[n-1]
			primary = row
		} else {
			last[row] = data[pos-1]
		}
	}
	return last, primary
}

// suffixArray 用倍增法构造后缀数组。旋转长度有限，旋转 i 的字典序等价于
// 后缀 i 在 data+data 上前 n 个字符的字典序，所以后缀数组同样给旋转排序。
// 每轮对二元组做两次稳定计数排序，整体 O(n log n) 时间、O(n) 空间。
func suffixArray(data []byte) []int {
	n := len(data)
	order := make([]int, n)
	rank := make([]int, n)
	tmpOrder := make([]int, n)
	tmpRank := make([]int, n)
	count := make([]int, max(n, 256)+1)

	for i := 0; i < n; i++ {
		rank[i] = int(data[i])
	}

	for step := 1; ; step <<= 1 {
		// 按第二关键字（循环后移 step 处的 rank，不存在则为 -1）稳定排序。
		for i := range count {
			count[i] = 0
		}
		for i := 0; i < n; i++ {
			count[rank[i]+1]++
		}
		for i := 1; i < len(count); i++ {
			count[i] += count[i-1]
		}
		for i := 0; i < n; i++ {
			idx := i - step
			if idx < 0 {
				idx += n
			}
			r := -1
			if i >= step {
				r = rank[idx]
			}
			p := count[r+1]
			count[r+1]++
			order[p] = idx
		}

		// 再按第一关键字稳定排序。
		for i := range count {
			count[i] = 0
		}
		for i := 0; i < n; i++ {
			count[rank[i]+1]++
		}
		for i := 1; i < len(count); i++ {
			count[i] += count[i-1]
		}
		for _, idx := range order {
			r := rank[idx]
			p := count[r+1]
			count[r+1]++
			tmpOrder[p] = idx
		}
		order, tmpOrder = tmpOrder, order

		// 压缩 rank。
		classes := 0
		for i, idx := range order {
			if i > 0 {
				prev := order[i-1]
				second := -1
				if idx+step < n {
					second = rank[idx+step]
				}
				prevSecond := -1
				if prev+step < n {
					prevSecond = rank[prev+step]
				}
				if rank[idx] != rank[prev] || second != prevSecond {
					classes++
				}
			}
			tmpRank[idx] = classes
		}
		rank, tmpRank = tmpRank, rank

		if classes == n-1 || step >= n {
			break
		}
	}
	return order
}

// Decode 用主索引与 LF mapping 一次重建原串，O(n) 时间、O(n) 空间。
// 不重排整张表，也不为每个位置复制整串。
func Decode(last []byte, primary int) ([]byte, error) {
	n := len(last)
	if n == 0 {
		if primary != 0 {
			return nil, fmt.Errorf("%w：primary=%d，允许范围 [0,0]，偏移 %d",
				ErrPrimaryOutOfRange, primary, 0)
		}
		return nil, nil
	}
	if primary < 0 || primary >= n {
		return nil, fmt.Errorf("%w：primary=%d，允许范围 [0,%d)，偏移 %d",
			ErrPrimaryOutOfRange, primary, n, 0)
	}

	// 字符频次与首列起始位置。
	var freq [256]int
	for _, b := range last {
		freq[b]++
	}
	var start [256]int
	total := 0
	for c := 0; c < 256; c++ {
		start[c] = total
		total += freq[c]
	}

	// lf[row]：最后一列第 row 行对应到第一列的行号。
	// 同一字符内部按出现顺序对应，用 seen 累计。
	lf := make([]int, n)
	var seen [256]int
	for row, b := range last {
		lf[row] = start[b] + seen[b]
		seen[b]++
	}

	out := make([]byte, n)
	row := primary
	for i := n - 1; i >= 0; i-- {
		out[i] = last[row]
		row = lf[row]
	}
	return out, nil
}

// RunLength 把数据编成 (字节, 游程长度 varint) 序列。
// 游程长度用无符号 LEB128（最多 10 字节）编码。空输入编码为空。
func RunLength(data []byte) []byte {
	n := len(data)
	if n == 0 {
		return nil
	}
	buf := make([]byte, 0, n)
	var uvar [binary.MaxVarintLen64]byte
	for i := 0; i < n; {
		b := data[i]
		run := 1
		for i+run < n && data[i+run] == b {
			run++
		}
		buf = append(buf, b)
		k := binary.PutUvarint(uvar[:], uint64(run))
		buf = append(buf, uvar[:k]...)
		i += run
	}
	return buf
}

// RunLengthDecode 解 (字节, 游程长度 varint) 序列。
// 遇到数据截断、varint 超过 10 字节、游程长度为 0 或长度溢出都报错，
// 错误信息带出错偏移（字节位置）。
func RunLengthDecode(encoded []byte) ([]byte, error) {
	out := make([]byte, 0, len(encoded))
	for offset := 0; offset < len(encoded); {
		b := encoded[offset]
		runStart := offset
		offset++
		run, consumed, err := readUvarint(encoded[offset:], offset)
		if err != nil {
			return nil, err
		}
		offset += consumed
		if run == 0 {
			return nil, fmt.Errorf("%w：偏移 %d", ErrRLEZeroRun, runStart)
		}
		if uint64(len(out))+run > uint64(maxInt) {
			return nil, fmt.Errorf("%w：游程累计长度溢出，偏移 %d", ErrRLELength, runStart)
		}
		for i := uint64(0); i < run; i++ {
			out = append(out, b)
		}
	}
	return out, nil
}

// readUvarint 读取无符号 LEB128，start 是数据在整段缓冲里的绝对偏移。
// 最多接受 10 个字节；第 10 个字节只允许最低位为 1。
func readUvarint(buf []byte, start int) (uint64, int, error) {
	var value uint64
	for i := 0; i < binary.MaxVarintLen64; i++ {
		if i >= len(buf) {
			return 0, 0, fmt.Errorf("%w：偏移 %d", ErrRLETruncated, start+i)
		}
		b := buf[i]
		if i == binary.MaxVarintLen64-1 && b > 1 {
			return 0, 0, fmt.Errorf("%w：偏移 %d", ErrRLEVarintTooLong, start+i)
		}
		value |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return value, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("%w：偏移 %d", ErrRLEVarintTooLong, start+binary.MaxVarintLen64)
}
