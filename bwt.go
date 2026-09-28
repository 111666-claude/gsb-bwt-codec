// Package bwtcodec 做 Burrows-Wheeler 变换与逆变换，只用标准库。
package bwtcodec

import (
	"bytes"
	"errors"
	"sort"
)

var (
	// ErrTooLong 表示输入超过朴素逆变换的长度上限。
	ErrTooLong = errors.New("bwtcodec: 文本太长，朴素逆变换只支持 4096 字节以内")
	// ErrNoRLE 表示游程编码还没有实现。
	ErrNoRLE = errors.New("bwtcodec: 游程编码还没有实现")
)

// Encode 返回 BWT 结果与主索引。
// 缺陷：为每个位置复制一整串旋转，内存 O(n²)。
func Encode(data []byte) ([]byte, int) {
	size := len(data)
	if size == 0 {
		return nil, 0
	}
	rotations := make([][]byte, size)
	for index := 0; index < size; index++ {
		rotation := make([]byte, size)
		copy(rotation, data[index:])
		copy(rotation[size-index:], data[:index])
		rotations[index] = rotation
	}
	sort.Slice(rotations, func(left, right int) bool {
		return bytes.Compare(rotations[left], rotations[right]) < 0
	})
	last := make([]byte, size)
	primary := 0
	for row, rotation := range rotations {
		last[row] = rotation[size-1]
		if bytes.Equal(rotation, data) {
			primary = row
		}
	}
	return last, primary
}

// Decode 缺陷：每轮把整张表重排一次（O(n²)），而且忘了用主索引，返回第 0 行。
func Decode(last []byte, primary int) ([]byte, error) {
	size := len(last)
	if size == 0 {
		return nil, nil
	}
	if size > 4096 {
		return nil, ErrTooLong
	}
	rows := make([][]byte, size)
	for index := 0; index < size; index++ {
		rows[index] = []byte{last[index]}
	}
	less := func(left, right int) bool {
		return bytes.Compare(rows[left], rows[right]) < 0
	}
	for round := 1; round < size; round++ {
		sort.Slice(rows, less)
		for index := 0; index < size; index++ {
			row := make([]byte, 0, len(rows[index])+1)
			row = append(row, last[index])
			rows[index] = append(row, rows[index]...)
		}
	}
	sort.Slice(rows, less)
	return rows[0], nil
}

// RunLength 缺陷：还没实现。
func RunLength(data []byte) []byte {
	return nil
}
