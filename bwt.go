// Package bwtcodec 做 Burrows-Wheeler 变换与逆变换，只用标准库。
package bwtcodec

import (
	"errors"
	"fmt"
)

func wrapOffset(err error, format string, args ...any) error {
	return fmt.Errorf("%w（%s）", err, fmt.Sprintf(format, args...))
}

var (
	// ErrPrimaryOutOfRange 表示主索引不在 [0, n) 范围内。
	ErrPrimaryOutOfRange = errors.New("bwtcodec: 主索引越界")
	// ErrRLETruncated 表示游程数据在游程中间结束。
	ErrRLETruncated = errors.New("bwtcodec: 游程数据截断")
	// ErrVarintTooLong 表示游程长度的 varint 编码超过 10 字节。
	ErrVarintTooLong = errors.New("bwtcodec: varint 超过 10 字节")
	// ErrZeroRunLength 某个游程的长度为 0。
	ErrZeroRunLength = errors.New("bwtcodec: 游程长度为 0")
	// ErrRLELengthMismatch 表示游程解码出的长度与 BWT 输入长度不一致。
	ErrRLELengthMismatch = errors.New("bwtcodec: 解出的长度与 BWT 输入长度不一致")
)

// Encode 返回 BWT 结果与主索引。
// 主索引是排序旋转表里等于原串的那一行下标；空输入返回空结果与主索引 0。
// 时间 O(n log n)，空间 O(n)，不为任何位置复制整串旋转。
func Encode(data []byte) ([]byte, int) {
	size := len(data)
	if size == 0 {
		return nil, 0
	}
	order := sortedRotations(data)

	last := make([]byte, size)
	primary := 0
	for row, start := range order {
		if start == 0 {
			primary = row
			last[row] = data[size-1]
		} else {
			last[row] = data[start-1]
		}
	}
	return last, primary
}

// sortedRotations 用倍增（prefix doubling）+ 计数排序返回所有循环旋转的起始下标，
// 按旋转字典序升序排列。相同旋转按下标打破平局，保证结果确定。
func sortedRotations(data []byte) []int {
	size := len(data)

	order := make([]int, size)
	rank := make([]int, size)
	{
		counts := make([]int, 256)
		for _, b := range data {
			counts[b]++
		}
		offset := 0
		for b := 0; b < 256; b++ {
			count := counts[b]
			counts[b] = offset
			offset += count
		}
		for i, b := range data {
			pos := counts[b]
			counts[b]++
			order[pos] = i
		}
		classes := 1
		rank[order[0]] = 0
		for i := 1; i < size; i++ {
			if data[order[i]] != data[order[i-1]] {
				classes++
			}
			rank[order[i]] = classes - 1
		}
		if classes == size {
			return order
		}
	}

	nextOrder := make([]int, size)
	nextRank := make([]int, size)
	counts := make([]int, size)
	for length := 1; length < size; length <<= 1 {
		classes := rank[order[size-1]] + 1

		// 第二关键字：旋转 [i+length, 2*length) 的名次。
		for i := range order {
			nextRank[i] = rank[(i+length)%size]
		}
		countingSort(order, nextOrder, nextRank, classes, counts)

		// 第一关键字：旋转 [i, length) 的名次。
		for _, i := range nextOrder {
			nextRank[i] = rank[i]
		}
		countingSort(nextOrder, order, nextRank, classes, counts)

		nextClasses := 1
		nextRank[order[0]] = 0
		for i := 1; i < size; i++ {
			cur, prev := order[i], order[i-1]
			if rank[cur] != rank[prev] ||
				rank[(cur+length)%size] != rank[(prev+length)%size] {
				nextClasses++
			}
			nextRank[cur] = nextClasses - 1
		}
		rank, nextRank = nextRank, rank
		if nextClasses == size || length >= size {
			break
		}
	}
	return order
}

// countingSort 按 keys 对 src 做稳定的计数排序，结果写入 dst。
// counts 是调用方提供的长度不小于 classCount 的计数数组。
func countingSort(src, dst, keys []int, classCount int, counts []int) {
	for i := 0; i < classCount; i++ {
		counts[i] = 0
	}
	for _, i := range src {
		counts[keys[i]]++
	}
	sum := 0
	for c := 0; c < classCount; c++ {
		count := counts[c]
		counts[c] = sum
		sum += count
	}
	for _, i := range src {
		key := keys[i]
		dst[counts[key]] = i
		counts[key]++
	}
}

// Decode 用主索引加 LF mapping 一次重建原串，时间 O(n)、空间 O(n)，
// 不重排整张表，也不为任何位置复制整串。
func Decode(last []byte, primary int) ([]byte, error) {
	size := len(last)
	if size == 0 {
		if primary != 0 {
			return nil, wrapOffset(ErrPrimaryOutOfRange, "n=0, primary=%d", primary)
		}
		return nil, nil
	}
	if primary < 0 || primary >= size {
		return nil, wrapOffset(ErrPrimaryOutOfRange, "n=%d, primary=%d", size, primary)
	}

	// 每个字节在第一列中的起始位置。
	var starts [256]int
	for _, b := range last {
		starts[b]++
	}
	total := 0
	for b := 0; b < 256; b++ {
		count := starts[b]
		starts[b] = total
		total += count
	}

	// LF[row] 是最后一列第 row 行在第一列中的下标：
	// 同字节按出现顺序一一对应，seen 记录这是该字节的第几次出现。
	lf := make([]int, size)
	var seen [256]int
	for row, b := range last {
		lf[row] = starts[b] + seen[b]
		seen[b]++
	}

	decoded := make([]byte, size)
	row := primary
	for i := size - 1; i >= 0; i-- {
		decoded[i] = last[row]
		row = lf[row]
	}
	return decoded, nil
}

// RunLength 把数据编成 (字节, 游程长度 varint) 序列。
// 游程长度用无符号 LEB128（每字节低 7 位，最高位表示后续还有字节）编码。
func RunLength(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	out := make([]byte, 0, len(data))
	var buf [10]byte
	for start := 0; start < len(data); {
		b := data[start]
		end := start + 1
		for end < len(data) && data[end] == b {
			end++
		}
		out = append(out, b)
		out = appendUvarint(out, buf[:], uint64(end-start))
		start = end
	}
	return out
}

// RunLengthDecode 解出游程序列，并校验长度与 wantLength（BWT 输入长度）一致。
// wantLength 传负数表示不校验长度。
func RunLengthDecode(data []byte, wantLength int) ([]byte, error) {
	total := 0
	offset := 0
	for offset < len(data) {
		runOffset := offset
		b := data[offset]
		offset++
		length, advance, err := readUvarint(data, offset)
		if err != nil {
			return nil, wrapOffset(err, "offset=%d", runOffset)
		}
		if length == 0 {
			return nil, wrapOffset(ErrZeroRunLength, "offset=%d, byte=%d", runOffset, b)
		}
		offset += advance
		if length > uint64(maxInt-total) {
			return nil, wrapOffset(ErrRLELengthMismatch, "offset=%d, run_length=%d", runOffset, length)
		}
		total += int(length)
	}
	if wantLength >= 0 && total != wantLength {
		return nil, wrapOffset(ErrRLELengthMismatch, "decoded=%d, want=%d", total, wantLength)
	}

	decoded := make([]byte, 0, total)
	offset = 0
	for offset < len(data) {
		b := data[offset]
		offset++
		length, advance, err := readUvarint(data, offset)
		if err != nil {
			return nil, wrapOffset(err, "offset=%d", offset-1)
		}
		offset += advance
		for i := uint64(0); i < length; i++ {
			decoded = append(decoded, b)
		}
	}
	return decoded, nil
}

const maxInt = int(^uint(0) >> 1)

// readUvarint 从 data[pos:] 读一个无符号 LEB128 整数，返回值、消耗字节数与错误。
// 超过 10 字节、或流在整数中间结束都算错误；偏移信息由调用方补充。
func readUvarint(data []byte, pos int) (uint64, int, error) {
	var value uint64
	for i := 0; ; i++ {
		if pos+i >= len(data) {
			return 0, i, ErrRLETruncated
		}
		b := data[pos+i]
		if i == 9 && b > 1 {
			return 0, i + 1, ErrVarintTooLong
		}
		value |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return value, i + 1, nil
		}
		if i == 9 {
			return 0, i + 1, ErrVarintTooLong
		}
	}
}

func appendUvarint(out, buf []byte, value uint64) []byte {
	n := 0
	for value >= 0x80 {
		buf[n] = byte(value) | 0x80
		value >>= 7
		n++
	}
	buf[n] = byte(value)
	return append(out, buf[:n+1]...)
}
