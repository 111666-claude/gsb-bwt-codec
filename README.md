# bwt-codec

Burrows-Wheeler 变换、逆变换与游程编码，只用 Go 标准库。

```
go test ./...
go run ./cmd/bwt-codec --text banana --roundtrip
go run ./cmd/bwt-codec --repeat 5000 --roundtrip --rle
```

## 口径（README 为准）

- **逆变换必须用主索引**：`--text banana --roundtrip` 按口径是 `roundtrip=true`，
  现在逆变换忘了用主索引、取的是第 0 行，给 `roundtrip=false`。
- **逆变换要一次构建**：用 LF mapping 在第一列与最后一列之间跳转，O(n) 时间、O(n) 空间；
  现在每轮都重排整张表（O(n²)），而且超过 4096 字节直接报「文本太长」。
- **游程编码**：`--rle` 把 BWT 结果做成「(字节, 游程长度 varint)」序列，能再解回原样；
  `--repeat 5000` 这种重复文本，游程编码后的字节数要 ≤ 原始字节数的 1/4。现在 `--rle` 直接报「游程编码还没有实现」。
- 主索引语义：排序后的旋转表里，等于原串的那一行的下标；空输入给主索引 0。
- 校验：主索引越界、游程数据截断、varint 超过 10 字节都要报错；解出的长度必须与 BWT 输入长度一致。
- 不变量：`Decode(Encode(data)) == data`（含空输入、单字节、全同字节、1 MB 随机字节）；
  同一输入重复变换结果逐字节相同。
- 规模：1 MB 输入要在几秒内完成，内存 O(n)；不许为每个位置复制一整串旋转。

## 现在的行为

- `Encode` 为每个位置复制一份完整旋转，内存 O(n²)。
- `Decode` 用「每轮重排整表」的朴素算法，还把主索引丢在一边，`banana` 的往返是 `false`；
  超过 4096 字节直接 `ErrTooLong`。
- `RunLength` 还没实现，`--rle` 直接失败。

## 计划新增的能力

```
--text 文本       要变换的文本
--repeat N        生成 N 个 'A' 作为输入
--roundtrip       变换后逆变换并比对
--rle             额外做游程编码
```

## 输出

```
bwt=nnbaaa primary=3 bytes=6
roundtrip=true
rle_bytes=4
```

## 目录

```
bwt.go              变换、逆变换与游程编码
cmd/bwt-codec       命令行入口
bwt_test.go         go test 用例
```
