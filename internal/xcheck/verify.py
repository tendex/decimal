import sys, decimal
from decimal import Decimal, Context

FLAGS = [(decimal.InvalidOperation, 1), (decimal.DivisionByZero, 2), (decimal.Overflow, 4),
         (decimal.Underflow, 8), (decimal.Inexact, 16)]
PARAMS = {"64": (16, 384), "128": (34, 6144)}

total = bad = skipped = 0
for line in sys.stdin:
    left, right = line.rstrip("\n").split(" -> ")
    width, op, mode, args = left.split(" ", 3)
    args = args.strip("[]").split(" ")
    got, gotflags = right.rsplit(" ", 1)
    prec, emax = PARAMS[width]
    ctx = Context(prec=prec, rounding=getattr(decimal, mode), Emin=1 - emax, Emax=emax, clamp=1,
                  traps=[], flags=[])
    ops = [Decimal(a) for a in args]
    want = getattr(ctx, op)(*ops)
    if op == "sqrt" and ctx.flags[decimal.Inexact]:
        # General Decimal Arithmetic always rounds sqrt half-even; IEEE 754
        # honours the rounding direction. Round a 120-digit root instead.
        hi = Context(prec=120, Emin=-999999, Emax=999999).sqrt(ops[0])
        ctx.clear_flags()
        want = ctx.plus(hi)
    wantflags = sum(bit for sig, bit in FLAGS if ctx.flags[sig])
    # General Decimal Arithmetic refuses remainders whose integer quotient
    # exceeds the precision; IEEE 754 does not.
    if op.startswith("remainder") and want.is_nan() and all(a.is_finite() for a in ops) and ops[1] != 0:
        skipped += 1
        continue
    total += 1
    if str(want) != got or wantflags != int(gotflags):
        bad += 1
        if bad <= 25:
            print(f"MISMATCH {left}\n   got  {got} [{gotflags}]\n   want {want} [{wantflags}]")
print(f"{total} checked, {skipped} skipped, {bad} mismatches")
sys.exit(1 if bad else 0)
