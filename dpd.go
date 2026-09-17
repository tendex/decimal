package decimal

// IEEE 754 defines two interchange encodings for each decimal format. This
// package computes in the binary integer decimal (BID) encoding, which is
// what Bits returns and what most software and the x86-64 and AArch64 ABIs
// use. The densely packed decimal (DPD) encoding, which packs the
// coefficient three digits to ten bits, is used by IBM hardware and
// software, and by wire formats derived from them. The functions in this
// file convert to and from it; they are the encodeDecimal and decodeDecimal
// operations of the standard.

// declets packs the low 3n decimal digits of v into n declets.
func declets(v uint64, n int) uint64 {
	var b uint64
	for i := 0; i < n; i++ {
		b |= uint64(bin2dpd[v%1000]) << (10 * i)
		v /= 1000
	}
	return b
}

// undeclets unpacks n declets.
func undeclets(b uint64, n int) uint64 {
	var v uint64
	for i := n - 1; i >= 0; i-- {
		v = v*1000 + uint64(dpd2bin[b>>(10*i)&0x3FF])
	}
	return v
}

// dpdCombination returns the 5-bit combination field for the leading digit
// and the two most significant bits of the biased exponent.
func dpdCombination(lead, expMSB uint64) uint64 {
	if lead < 8 {
		return expMSB<<3 | lead
	}
	return 0b11000 | expMSB<<1 | lead&1
}

// dpdSplit is the inverse of dpdCombination for a finite number.
func dpdSplit(comb uint64) (lead, expMSB uint64) {
	if comb>>3 != 0b11 {
		return comb & 7, comb >> 3
	}
	return 8 | comb&1, comb >> 1 & 3
}

// New64FromDPD returns the Decimal64 with the IEEE 754 densely packed
// decimal interchange encoding b. Every bit pattern is valid.
func New64FromDPD(b uint64) Decimal64 {
	n := num{neg: b>>63 != 0}
	comb := b >> 58 & 0x1F
	switch {
	case comb == 0b11110:
		n.kind = infinite
	case comb == 0b11111:
		n.kind = quietNaN
		if b>>57&1 != 0 {
			n.kind = signalingNaN
		}
		n.coef = undeclets(b, 5)
	default:
		lead, expMSB := dpdSplit(comb)
		n.coef = lead*1e15 + undeclets(b, 5)
		n.exp = int32(expMSB<<8|b>>50&0xFF) - bias64
	}
	return pack64(n)
}

// DPD returns the IEEE 754 densely packed decimal interchange encoding of x,
// which is always canonical.
func (x Decimal64) DPD() uint64 {
	n := x.unpack()
	var b uint64
	switch n.kind {
	case finite:
		e := uint64(n.exp + bias64)
		b = dpdCombination(n.coef/1e15, e>>8)<<58 | e&0xFF<<50 | declets(n.coef%1e15, 5)
	case infinite:
		b = 0b11110 << 58
	case quietNaN:
		b = 0b11111<<58 | declets(n.coef, 5)
	default:
		b = 0b11111<<58 | 1<<57 | declets(n.coef, 5)
	}
	if n.neg {
		b |= sign64
	}
	return b
}
