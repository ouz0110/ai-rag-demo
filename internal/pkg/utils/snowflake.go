package utils

import (
	"os"

	sf "github.com/StarryLab/tsid.go"
	"github.com/bwmarrin/snowflake"
	"github.com/google/uuid"
)

var snowflakeNode *snowflake.Node

func InitSnowflakeNode() error {
	hn, _ := os.Hostname()
	sum := 0
	for _, c := range hn {
		sum += int(c)
	}
	nodeNum := int64(sum % 1024) // TODO 暂时使用该方式

	var err error
	snowflakeNode, err = snowflake.NewNode(nodeNum)
	return err
}

// GenerateSnowflakeID 生成 Snowflake ID。
// Int64  ID: 1668089848088498176
// String ID: 1668089848088498176
// Base2  ID: 1011100100110001111101110000100000000110000000001000000000000
// Base32 ID: bqjt6hrycyryy
// Base36 ID: co8oi16kbchs
// Base58 ID: 4SzwTJbfzuU
// Base64 ID: MTY2ODA4OTg0ODA4ODQ5ODE3Ng==
func GenerateSnowflakeID() snowflake.ID {
	return snowflakeNode.Generate()
}

var node *sf.Builder

func InitTSID(h, n int64) {
	b, err := sf.Snowflake(h, n)
	if err != nil {
		panic(err)
	}

	node = b
}

// GenOpenID 生成open id（基于 tsid.go Snowflake，单调递增、可排序，可解析时间戳）
func GenOpenID() string {
	return node.Next().String()
}

// NewUUID 生成 UUID（完全随机，无序，基于 google/uuid）
func NewUUID() string {
	return uuid.NewString()
}
