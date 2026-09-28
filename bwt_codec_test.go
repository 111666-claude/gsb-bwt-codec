package bwtcodec

import (
	"bytes"
	"errors"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBananaGolden(t *testing.T) {
	last, primary := Encode([]byte("banana"))
	if got := string(last); got != "nnbaaa" {
		t.Fatalf("banana 的 BWT 应为 nnbaaa，实际 %q", got)
	}
	if primary != 3 {
		t.Fatalf("banana 的主索引应为 3，实际 %d", primary)
	}
	decoded, err := Decode(last, primary)
	if err != nil {
		t.Fatalf("Decode 出错：%v", err)
	}
	if string(decoded) != "banana" {
		t.Fatalf("banana 往返失败：%q", decoded)
	}
}

func TestRoundtripCases(t *testing.T) {
	cases := [][]byte{
		nil,
		{},
		{'q'},
		[]byte("ab"),
		[]byte("ba"),
		[]byte("AAAA"),
		bytes.Repeat([]byte{'Z'}, 1000),
		[]byte("mississippi"),
		[]byte("aaaaaaaaaab"),
		[]byte("abababababab"),
		[]byte("a\x00b\x00c\xff\xfe"),
	}
	for _, data := range cases {
		last, primary := Encode(data)
		if len(last) != len(data) {
			t.Fatalf("BWT 长度应与输入一致：%d vs %d", len(last), len(data))
		}
		if !sameMultiset(last, data) {
			t.Fatalf("BWT 与输入不是同一字节多重集：%q", data)
		}
		if len(data) > 0 && (primary < 0 || primary >= len(data)) {
			t.Fatalf("主索引越界：%d / %d", primary, len(data))
		}
		decoded, err := Decode(last, primary)
		if err != nil {
			t.Fatalf("Decode(%q) 出错：%v", data, err)
		}
		if !bytes.Equal(decoded, data) {
			t.Fatalf("往返失败：输入 %q，解出 %q", data, decoded)
		}

		encoded := RunLength(last)
		decodedLast, err := RunLengthDecode(encoded, len(last))
		if err != nil {
			t.Fatalf("游程往返出错（%q）：%v", data, err)
		}
		if !bytes.Equal(decodedLast, last) {
			t.Fatalf("游程往返与 BWT 不一致：%q", data)
		}
	}
}

func TestRoundtripEmptyPrimary(t *testing.T) {
	last, primary := Encode(nil)
	if len(last) != 0 || primary != 0 {
		t.Fatalf("空输入应为空结果与主索引 0：%v %d", last, primary)
	}
}

func TestSingleBytePrimary(t *testing.T) {
	last, primary := Encode([]byte{'q'})
	if primary != 0 || string(last) != "q" {
		t.Fatalf("单字节输入主索引应为 0：%q %d", last, primary)
	}
}

func TestRoundtripPseudorandom1MB(t *testing.T) {
	const size = 1 << 20
	rng := rand.New(rand.NewSource(42))
	data := make([]byte, size)
	rng.Read(data)

	start := time.Now()
	last, primary := Encode(data)
	encElapsed := time.Since(start)
	if !sameMultiset(last, data) {
		t.Fatalf("1 MB 输入的 BWT 不是同一字节多重集")
	}

	start = time.Now()
	decoded, err := Decode(last, primary)
	if err != nil {
		t.Fatalf("1 MB Decode 出错：%v", err)
	}
	decElapsed := time.Since(start)
	if !bytes.Equal(decoded, data) {
		t.Fatalf("1 MB 往返失败")
	}
	t.Logf("1 MB 往返：encode=%s decode=%s", encElapsed, decElapsed)
	if encElapsed > 10*time.Second || decElapsed > 10*time.Second {
		t.Fatalf("1 MB 输入应在几秒内完成：encode=%s decode=%s", encElapsed, decElapsed)
	}

	encoded := RunLength(last)
	decodedLast, err := RunLengthDecode(encoded, len(last))
	if err != nil {
		t.Fatalf("1 MB 游程往返出错：%v", err)
	}
	if !bytes.Equal(decodedLast, last) {
		t.Fatalf("1 MB 游程往返不一致")
	}
}

func TestPrimaryOutOfRange(t *testing.T) {
	cases := []struct {
		name    string
		last    []byte
		primary int
	}{
		{"空输入主索引非零", nil, 1},
		{"主索引等于长度", []byte("ab"), 2},
		{"主索引为负", []byte("ab"), -1},
		{"主索引远超范围", []byte("ab"), 99},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(tc.last, tc.primary)
			if !errors.Is(err, ErrPrimaryOutOfRange) {
				t.Fatalf("应报主索引越界，实际 %v", err)
			}
			if !strings.Contains(err.Error(), "primary=") {
				t.Fatalf("错误信息应给出位置信息：%v", err)
			}
		})
	}
}

func TestRLECompressionRepeat5000(t *testing.T) {
	data := bytes.Repeat([]byte{'A'}, 5000)
	last, primary := Encode(data)
	encoded := RunLength(last)
	decoded, err := Decode(last, primary)
	if err != nil || !bytes.Equal(decoded, data) {
		t.Fatalf("重复文本往返失败：%v", err)
	}
	if len(encoded) > len(data)/4 {
		t.Fatalf("游程编码后 %d 字节，应不超过原始的 1/4（%d）", len(encoded), len(data)/4)
	}
	decodedLast, err := RunLengthDecode(encoded, len(last))
	if err != nil {
		t.Fatalf("游程解码出错：%v", err)
	}
	if !bytes.Equal(decodedLast, last) {
		t.Fatalf("游程往返不一致")
	}
}

func TestRLERoundtripRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for iter := 0; iter < 50; iter++ {
		data := make([]byte, rng.Intn(2000))
		alphabet := 1 + rng.Intn(4)
		for i := range data {
			data[i] = byte(rng.Intn(alphabet))
		}
		last, _ := Encode(data)
		encoded := RunLength(last)
		decoded, err := RunLengthDecode(encoded, len(last))
		if err != nil {
			t.Fatalf("iter %d：游程解码出错：%v", iter, err)
		}
		if !bytes.Equal(decoded, last) {
			t.Fatalf("iter %d：游程往返不一致", iter)
		}
	}
}

func TestRLEErrors(t *testing.T) {
	tenContinuation := bytes.Repeat([]byte{0x80}, 10)
	cases := []struct {
		name   string
		data   []byte
		want   error
		offset string
	}{
		{"游程后缺少长度", []byte{'a'}, ErrRLETruncated, "offset=0"},
		{"长度编码后被截断", []byte{'a', 0x80}, ErrRLETruncated, "offset=0"},
		{"第二个游程截断", []byte{'a', 1, 'b'}, ErrRLETruncated, "offset=2"},
		{"varint 十个延续位", append([]byte{'a'}, tenContinuation...), ErrVarintTooLong, "offset=0"},
		{"varint 超过十字节", append(append([]byte{'a'}, tenContinuation...), 0x00), ErrVarintTooLong, "offset=0"},
		{"游程长度为零", []byte{'a', 0x00}, ErrZeroRunLength, "offset=0"},
		{"后续游程长度为零", []byte{'a', 1, 'b', 0x00}, ErrZeroRunLength, "offset=2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunLengthDecode(tc.data, -1)
			if !errors.Is(err, tc.want) {
				t.Fatalf("应报 %v，实际 %v", tc.want, err)
			}
			if !strings.Contains(err.Error(), tc.offset) {
				t.Fatalf("错误信息应给出偏移 %s，实际 %v", tc.offset, err)
			}
		})
	}
}

func TestRLELengthMismatch(t *testing.T) {
	encoded := RunLength([]byte("aa"))
	if _, err := RunLengthDecode(encoded, 3); !errors.Is(err, ErrRLELengthMismatch) {
		t.Fatalf("长度不一致应报错，实际 %v", err)
	}
}

func TestMatchesNaiveRandom200(t *testing.T) {
	rng := rand.New(rand.NewSource(20240928))
	for iter := 0; iter < 200; iter++ {
		var data []byte
		switch iter % 6 {
		case 0:
			data = make([]byte, rng.Intn(2001))
			alphabet := 1 + rng.Intn(256)
			for i := range data {
				data[i] = byte(rng.Intn(alphabet))
			}
		case 1:
			data = bytes.Repeat([]byte{byte(rng.Intn(256))}, rng.Intn(2001))
		case 2:
			data = make([]byte, rng.Intn(2001))
			pattern := []byte{byte(rng.Intn(256)), byte(rng.Intn(256)), byte(rng.Intn(3))}
			for i := range data {
				data[i] = pattern[i%len(pattern)]
			}
		case 3:
			data = make([]byte, 2000)
			for i := range data {
				data[i] = byte(rng.Intn(2))
			}
		case 4:
			data = make([]byte, rng.Intn(50))
			for i := range data {
				data[i] = byte(rng.Intn(256))
			}
		default:
			n := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 255, 256, 257}[rng.Intn(17)]
			data = make([]byte, n)
			for i := range data {
				data[i] = byte(rng.Intn(4))
			}
		}

		last, primary := Encode(data)
		naiveLast, naivePrimary := encodeNaive(data)
		if !bytes.Equal(last, naiveLast) {
			t.Fatalf("iter %d（长度 %d）：BWT 与朴素实现不一致\n got %v\nwant %v", iter, len(data), last, naiveLast)
		}
		if primary != naivePrimary {
			t.Fatalf("iter %d（长度 %d）：主索引与朴素实现不一致 %d vs %d", iter, len(data), primary, naivePrimary)
		}
	}
}

// encodeNaive 把所有旋转拷出来排序，作为朴素对照实现。
func encodeNaive(data []byte) ([]byte, int) {
	size := len(data)
	if size == 0 {
		return nil, 0
	}
	type rotation struct {
		start int
		bytes []byte
	}
	rotations := make([]rotation, size)
	for start := 0; start < size; start++ {
		buf := make([]byte, size)
		copy(buf, data[start:])
		copy(buf[size-start:], data[:start])
		rotations[start] = rotation{start, buf}
	}
	sort.Slice(rotations, func(i, j int) bool {
		cmp := bytes.Compare(rotations[i].bytes, rotations[j].bytes)
		if cmp != 0 {
			return cmp < 0
		}
		return rotations[i].start < rotations[j].start
	})
	last := make([]byte, size)
	primary := 0
	for row, rotation := range rotations {
		last[row] = rotation.bytes[size-1]
		if rotation.start == 0 {
			primary = row
		}
	}
	return last, primary
}

func sameMultiset(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	counts := [256]int{}
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		counts[v]--
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
}
