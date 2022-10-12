package main

import (
	"bytes"
	"fmt"
	"github.com/willf/pad"
	"math/big"
	"math/rand"
	"strings"
)

type ALPH struct {
	Amount *big.Int
}

var (
	OneQuintillionString = "1000000000000000000"
	OneQuintillionInt64  = int64(1000000000000000000)
	OneBillionInt64      = int64(1000000000)
	CoinInOneALPH        = new(big.Int).SetInt64(OneQuintillionInt64)
	CoinInNanoALPH       = new(big.Int).SetInt64(OneBillionInt64)
	//N = "א"
	N = "ALPH"
)

func ALPHFromALPHString(amount string) (ALPH, bool) {
	split := strings.Split(amount, ".")
	if len(split) == 1 {
		alphAmount, ok := new(big.Int).SetString(amount, 10)
		if ok {
			coinAmount := new(big.Int).Mul(alphAmount, CoinInOneALPH)
			return ALPH{Amount: coinAmount}, true
		}
	} else if len(split) == 2 {
		decimals := pad.Right(split[1], 18, "0")
		coinAmount, ok := new(big.Int).SetString(strings.Join([]string{split[0], decimals}, ""), 10)
		if ok {
			return ALPH{Amount: coinAmount}, true
		}
	}
	return ALPH{}, false
}

func ALPHFromCoinString(amount string) (ALPH, bool) {
	alphAmount, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return ALPH{}, false
	}
	return ALPH{Amount: alphAmount}, true
}

func (alph ALPH) Add(other ALPH) ALPH {
	c := new(big.Int)
	c.Add(alph.Amount, other.Amount)
	return ALPH{Amount: c}
}

func (alph ALPH) Subtract(other ALPH) ALPH {
	c := new(big.Int)
	c.Sub(alph.Amount, other.Amount)
	return ALPH{Amount: c}
}

func (alph ALPH) Multiply(multiplier int64) ALPH {
	c := new(big.Int)
	m := new(big.Int).SetInt64(multiplier)
	c.Mul(alph.Amount, m)
	return ALPH{Amount: c}
}

func (alph ALPH) Divide(divider int64) ALPH {
	c := new(big.Int)
	m := new(big.Int).SetInt64(divider)
	c.Div(alph.Amount, m)
	return ALPH{Amount: c}
}

func (alph ALPH) Cmp(other ALPH) int {
	return alph.Amount.Cmp(other.Amount)
}

func (alph ALPH) String() string {
	if alph.Amount == nil {
		return "0"
	}
	return alph.Amount.String()
}

func (alph ALPH) PrettyString() string {
	if alph.Amount == nil {
		return "0"
	}
	if alph.Amount.Cmp(CoinInNanoALPH) > 0 {
		return fmt.Sprintf("%s%s", strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.9f", alph.FloatALPH()), "0"), "."), N)
	}
	return alph.Amount.String()
}

func (alph ALPH) FloatALPH() float64 {
	c := new(big.Int)
	nanoAFL := c.Div(alph.Amount, CoinInNanoALPH).Int64()
	return float64(nanoAFL) / float64(OneBillionInt64)
}

func RandomALPHAmount(upperLimit int) ALPH {
	unit := rand.Intn(upperLimit)
	decimals := rand.Intn(int(OneBillionInt64))
	rAmountStr := fmt.Sprintf("%d.%d", unit, decimals)
	alph, _ := ALPHFromALPHString(rAmountStr)
	return alph
}

func RandomNanoALPHAmount(upperLimit int) ALPH {
	nanoALPH := rand.Intn(upperLimit)
	c := new(big.Int).SetInt64(int64(nanoALPH))
	m := new(big.Int).Mul(c, CoinInNanoALPH)
	return ALPH{Amount: m}
}

func (alph ALPH) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	str := alph.Amount.String()
	buffer.WriteString(str)
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

func (alph *ALPH) UnmarshalJSON(b []byte) error {
	if len(b) < 2 {
		return fmt.Errorf("NaN")
	}
	alph.Amount = new(big.Int)
	err := alph.Amount.UnmarshalJSON(b[1 : len(b)-1])
	return err
}

func ToNanoALPH(alph ALPH) int {
	m := new(big.Int).Div(alph.Amount, CoinInNanoALPH)
	return int(m.Int64())
}
