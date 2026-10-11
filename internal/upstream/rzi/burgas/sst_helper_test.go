package burgas

const chunkMax = 8000

// encodeSST writes the SST record and as many CONTINUE records as needed to
// keep every record under the BIFF8 limit. Strings split in the middle of
// their characters; the continuation starts with a repeated flag byte.
// splitStr and splitChar force one extra split.
func encodeSST(strs []string, o bookOpts) []byte {
	var chunks [][]byte
	cur := append(le32(uint32(len(strs))), le32(uint32(len(strs)))...)
	for i, str := range strs {
		lat := o.latin1 && isASCII(str)
		flag, unit := byte(1), 2
		chars := utf16le(str)
		count := len(chars) / 2
		if lat {
			flag, unit, chars = 0, 1, []byte(str)
			count = len(str)
		}
		if len(cur)+3 > chunkMax {
			chunks, cur = append(chunks, cur), nil
		}
		cur = append(cur, le16(count)...)
		cur = append(cur, flag)
		forced := o.splitChar > 0 && i == o.splitStr
		for len(chars) > 0 {
			take := min(len(chars), (chunkMax-len(cur))/unit*unit)
			if forced {
				take = min(take, o.splitChar*unit)
			}
			cur = append(cur, chars[:take]...)
			chars = chars[take:]
			if len(chars) == 0 {
				break
			}
			if forced && o.flipTail && flag == 0 {
				// The rest of an ASCII string continues as UTF-16.
				chars = utf16le(string(chars))
				flag, unit = 1, 2
			}
			forced = false
			chunks, cur = append(chunks, cur), []byte{flag}
		}
	}
	chunks = append(chunks, cur)
	out := rec(recSST, chunks[0])
	for _, c := range chunks[1:] {
		out = append(out, rec(recContinue, c)...)
	}
	return out
}
