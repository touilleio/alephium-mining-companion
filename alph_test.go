package main

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestALPHConstruct(t *testing.T) {
	a1, ok := ALPHFromALPHString("12")
	assert.True(t, ok)
	assert.Equal(t, 12.000000000, a1.FloatALPH())
	assert.Equal(t, fmt.Sprintf("12.000000000%s", N), a1.PrettyString())
	assert.Equal(t, "12000000000000000000", a1.String())

	a2, ok := ALPHFromCoinString("12")
	assert.True(t, ok)
	assert.Equal(t, "12", a2.PrettyString())
	assert.Equal(t, "12", a2.String())
}

func TestZeroALPH(t *testing.T) {
	a1 := ALPH{}
	assert.Equal(t, "0", a1.String())
}

func TestALPHConstructWithDot(t *testing.T) {
	a1, ok := ALPHFromALPHString("12.12")
	assert.True(t, ok)
	assert.Equal(t, 12.120000000, a1.FloatALPH())
}

func TestALPHAdd(t *testing.T) {
	a1, ok := ALPHFromALPHString("10.1")
	assert.True(t, ok)
	a2, ok := ALPHFromALPHString("2.02")
	assert.True(t, ok)
	a3, ok := ALPHFromALPHString("12.12")
	assert.True(t, ok)

	res := a1.Add(a2)
	assert.Equal(t, 0, res.Cmp(a3))
	assert.Equal(t, res, a3)
}

func TestALPHSub(t *testing.T) {
	a1, ok := ALPHFromALPHString("10")
	assert.True(t, ok)
	a2, ok := ALPHFromALPHString("2")
	assert.True(t, ok)
	a3, ok := ALPHFromALPHString("12")
	assert.True(t, ok)

	res := a3.Subtract(a2)
	assert.Equal(t, 0, res.Cmp(a1))
	assert.Equal(t, res, a1)
}

func TestALPHMul(t *testing.T) {
	a1, ok := ALPHFromALPHString("10")
	assert.True(t, ok)
	a2, ok := ALPHFromALPHString("2")
	assert.True(t, ok)
	a3, ok := ALPHFromALPHString("12")
	assert.True(t, ok)

	res := a2.Multiply(5)
	assert.Equal(t, 0, res.Cmp(a1))
	assert.Equal(t, res, a1)
	res = a2.Multiply(6)
	assert.Equal(t, 0, res.Cmp(a3))
	assert.Equal(t, res, a3)
}

func TestALPHDiv(t *testing.T) {
	a1, ok := ALPHFromALPHString("10")
	assert.True(t, ok)
	a2, ok := ALPHFromALPHString("2")
	assert.True(t, ok)
	a3, ok := ALPHFromALPHString("12")
	assert.True(t, ok)

	res := a1.Divide(5)
	assert.Equal(t, 0, res.Cmp(a2))
	assert.Equal(t, res, a2)
	res = a3.Divide(6)
	assert.Equal(t, 0, res.Cmp(a2))
	assert.Equal(t, res, a2)
}

func TestALPHDecode(t *testing.T) {
	a1, ok := ALPHFromALPHString("10")
	assert.True(t, ok)
	a2, ok := ALPHFromALPHString("2")
	assert.True(t, ok)
	a3, ok := ALPHFromALPHString("12")
	assert.True(t, ok)

	res := a1.Divide(5)
	assert.Equal(t, 0, res.Cmp(a2))
	assert.Equal(t, res, a2)
	res = a3.Divide(6)
	assert.Equal(t, 0, res.Cmp(a2))
	assert.Equal(t, res, a2)
}

func TestRoundAmount(t *testing.T) {
	rand.Seed(time.Now().UnixNano() + int64(os.Getpid()))
	alph := RandomALPHAmount(100)
	fmt.Printf("%s", alph.PrettyString())

	TenALPH, ok := ALPHFromALPHString("10")
	assert.True(t, ok)

	// just to ensure at least 10 ALPH
	alph = alph.Add(TenALPH)
	assert.True(t, alph.Cmp(TenALPH) > 0)
	var finalTxAmount ALPH
	fiveALPH := TenALPH.Divide(2)
	txAmount := alph.Subtract(fiveALPH)
	if txAmount.Cmp(TenALPH) > 0 {
		finalTxAmount = TenALPH
	} else {
		finalTxAmount = txAmount
	}

	assert.True(t, finalTxAmount.Cmp(TenALPH) <= 0)
}

func TestNanoAmount(t *testing.T) {
	rand.Seed(time.Now().UnixNano() + int64(os.Getpid()))
	alph := RandomNanoALPHAmount(int(OneBillionInt64))
	fmt.Printf("%s\n", alph.PrettyString())
	fmt.Printf("%.4f\n", alph.FloatALPH())

	alph = RandomNanoALPHAmount(10000000)
	fmt.Printf("%s\n", alph.PrettyString())
}

type TestJsonStruct struct {
	Amount ALPH `json:"Amount"`
}

func TestJSON(t *testing.T) {
	rand.Seed(time.Now().UnixNano() + int64(os.Getpid()))
	alph := RandomNanoALPHAmount(int(OneBillionInt64))

	j1 := TestJsonStruct{
		Amount: alph,
	}

	b, err := json.Marshal(j1)
	assert.Nil(t, err)

	fmt.Printf("-->%s\n", string(b))

	j2 := &TestJsonStruct{}
	err = json.Unmarshal(b, j2)
	assert.Nil(t, err)

	fmt.Printf("%s\n", j2.Amount.PrettyString())
}
