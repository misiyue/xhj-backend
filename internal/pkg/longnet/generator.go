package longnet

import (
	"context"
	"sync/atomic"

	"github.com/bwmarrin/snowflake"
	"github.com/redis/go-redis/v9"
)

type IdGenerator interface {
	// IdGen 获取自增ID
	IdGen() int64
}

type SnowflakeGenerator struct {
	snowflake *snowflake.Node
}

// NewSnowflakeGenerator creates a new SnowflakeGenerator with a unique node ID
// For High Availability support, it uses Redis to assign unique node IDs across instances
func NewSnowflakeGenerator(rds *redis.Client) *SnowflakeGenerator {
	// Use Redis counter to get a unique node ID for this instance
	// This ensures different instances get different node IDs, preventing ID collisions
	res := rds.Incr(context.Background(), "snowflake_work_node")
	if err := res.Err(); err != nil {
		panic(err)
	}
	
	nodeId := res.Val() % 1024 // Snowflake supports 0-1023 node IDs

	node, err := snowflake.NewNode(nodeId)
	if err != nil {
		panic(err)
	}

	return &SnowflakeGenerator{
		snowflake: node,
	}
}

func (s *SnowflakeGenerator) IdGen() int64 {
	return s.snowflake.Generate().Int64()
}

type AutoIdGenerator struct {
	lastId int64
}

func NewAutoIdGenerator() *AutoIdGenerator {
	return &AutoIdGenerator{
		lastId: 0,
	}
}

func (a *AutoIdGenerator) IdGen() int64 {
	return atomic.AddInt64(&a.lastId, 1)
}
