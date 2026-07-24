package utils

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/shopspring/decimal"
)

// Money 金钱
type Money struct {
	CurrencyCode string          `json:"currency_code"`
	Value        decimal.Decimal `json:"value"`
}

// NewMoney 创建 Money 对象，参数 cent 为分
func NewMoney(cent int64, currencyCode string) *Money {
	return &Money{
		CurrencyCode: currencyCode,
		Value:        decimal.NewFromBigRat(big.NewRat(cent, 100), 2),
	}
}

func (a *Money) IsZero() bool {
	return a.Value.IsZero()
}

func (a *Money) IsMoneyEqual(b *Money) (bool, error) {
	if a.CurrencyCode != b.CurrencyCode {
		return false, nil
	}
	return a.Value.Equal(b.Value), nil
}

func (a *Money) AddMoney(ms ...*Money) error {
	if len(ms) == 0 {
		return nil
	}
	for _, m := range ms {
		if m.CurrencyCode != a.CurrencyCode {
			return fmt.Errorf("currency mismatch")
		}
		a.Value = a.Value.Add(m.Value)
	}
	return nil
}

func (a *Money) SubMoney(b *Money) error {
	if a.CurrencyCode != b.CurrencyCode {
		return fmt.Errorf("currency mismatch")
	}
	a.Value = a.Value.Sub(b.Value)
	return nil
}

func (a *Money) IsMoneyGreater(b *Money) (bool, error) {
	if a.CurrencyCode != b.CurrencyCode {
		return false, fmt.Errorf("currency mismatch")
	}
	return a.Value.GreaterThan(b.Value), nil
}

func (a *Money) IsIllegalMoney() bool {
	return a.Value.LessThan(decimal.NewFromInt(0))
}

func (a *Money) CompareMoney(b *Money) (int, error) {
	if a.CurrencyCode != b.CurrencyCode {
		return 0, fmt.Errorf("currency mismatch")
	}
	return a.Value.Cmp(b.Value), nil
}

func (a *Money) String() string {
	return a.Value.String()
}

func (a *Money) StringFixed(places int32) string {
	return a.Value.StringFixed(places)
}

func (a *Money) Int64() int64 {
	return a.Value.CoefficientInt64()
}

func (a *Money) Uint32() uint32 {
	return uint32(a.Value.CoefficientInt64())
}

// Format 格式化为 10,000.00格式
func (a *Money) Format() string {
	// 1. 获取固定 2 位小数的字符串，例如 "10000.00"
	str := a.StringFixed(2)

	// 2. 分离整数部分和小数部分
	parts := strings.Split(str, ".")
	intPart := parts[0]
	decPart := parts[1]

	// 3. 处理负号
	sign := ""
	if strings.HasPrefix(intPart, "-") {
		sign = "-"
		intPart = intPart[1:]
	}

	// 4. 对整数部分添加千分位
	// 逻辑：从后往前遍历，每 3 位插入一个逗号
	var result strings.Builder
	n := len(intPart)
	// 计算需要插入逗号的个数
	commaCount := (n - 1) / 3

	// 预分配容量以提高性能
	result.Grow(n + commaCount + 1 + len(decPart))

	result.WriteString(sign)

	// 处理头部不够 3 位的部分 (例如 10,000 中的 10)
	head := n % 3
	if head > 0 {
		result.WriteString(intPart[:head])
		if commaCount > 0 {
			result.WriteString(",")
		}
	}

	// 处理剩余的 3 位一组
	for i := head; i < n; i += 3 {
		result.WriteString(intPart[i : i+3])
		if i+3 < n {
			result.WriteString(",")
		}
	}

	// 5. 拼接小数部分
	result.WriteString(".")
	result.WriteString(decPart)

	return result.String()
}

// ServiceFee 计算服务费
// rateBp: 费率百分比（如 0.05 代表 0.05%）
// return: 服务费（分）
func (a *Money) ServiceFee(rateBp float64) *Money {
	amount := decimal.NewFromInt(a.Int64())

	// 2. 将 float64 费率转换为 decimal
	// 注意：0.05% = 0.05 / 100 = 0.0005
	rate := decimal.NewFromFloat(rateBp).Div(decimal.NewFromInt(100))

	// 3. 计算乘积
	fee := amount.Mul(rate)

	// 4. 四舍五入保留 0 位小数 (Round默认就是四舍五入)
	// 如果需要银行家舍入法等其他策略，decimal 也支持
	return NewMoney(fee.Round(0).IntPart(), a.CurrencyCode)
}
