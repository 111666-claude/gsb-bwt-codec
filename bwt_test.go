package bwtcodec

import (
	"bytes"
	"sort"
	"testing"
)

func TestEncodeEmpty(t *testing.T) {
	last, primary := Encode(nil)
	if len(last) != 0 || primary != 0 {
		t.Fatalf("空输入应为空：%v %d", last, primary)
	}
	decoded, err := Decode(nil, 0)
	if err != nil || len(decoded) != 0 {
		t.Fatalf("空输入应解出空：%v %v", decoded, err)
	}
}

func TestEncodeKeepsLengthAndBytes(t *testing.T) {
	raw := []byte("banana")
	last, primary := Encode(raw)
	if len(last) != len(raw) {
		t.Fatalf("BWT 结果长度应与输入一致：%d", len(last))
	}
	if primary < 0 || primary >= len(raw) {
		t.Fatalf("主索引应在范围内：%d", primary)
	}
	sortedRaw := append([]byte(nil), raw...)
	sortedLast := append([]byte(nil), last...)
	sort.Slice(sortedRaw, func(left, right int) bool { return sortedRaw[left] < sortedRaw[right] })
	sort.Slice(sortedLast, func(left, right int) bool { return sortedLast[left] < sortedLast[right] })
	if !bytes.Equal(sortedRaw, sortedLast) {
		t.Fatalf("BWT 结果是输入的排列：%v vs %v", sortedRaw, sortedLast)
	}
}

func TestEncodeIsDeterministic(t *testing.T) {
	first, firstPrimary := Encode([]byte("mississippi"))
	second, secondPrimary := Encode([]byte("mississippi"))
	if !bytes.Equal(first, second) || firstPrimary != secondPrimary {
		t.Fatalf("同一输入应该给同样的结果：%v/%d vs %v/%d", first, firstPrimary, second, secondPrimary)
	}
}

func TestDecodeKeepsProvidedLength(t *testing.T) {
	last, primary := Encode([]byte("ab"))
	decoded, err := Decode(last, primary)
	if err != nil {
		t.Fatalf("Decode 出错：%v", err)
	}
	if len(decoded) != len(last) {
		t.Fatalf("解出的长度应与 BWT 结果一致：%d", len(decoded))
	}
}

func TestDecodeSingleByte(t *testing.T) {
	decoded, err := Decode([]byte{'q'}, 0)
	if err != nil {
		t.Fatalf("Decode 出错：%v", err)
	}
	if !bytes.Equal(decoded, []byte{'q'}) {
		t.Fatalf("单字节输入应解出同一个字节：%v", decoded)
	}
}
