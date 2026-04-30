package utils

import (
	"context"
	"sync"

	"github.com/bwmarrin/snowflake"
	"github.com/redis/go-redis/v9"
)

var (
	snowflakeNode *snowflake.Node
	once          sync.Once
)

// InitSnowflake initializes the snowflake node with a unique node ID from Redis
// This must be called during application startup for High Availability support
func InitSnowflake(rds *redis.Client) error {
	var initErr error
	once.Do(func() {
		// Use Redis counter to get a unique node ID for this instance
		// This ensures different instances get different node IDs, preventing ID collisions
		res := rds.Incr(context.Background(), "snowflake_work_node")
		if err := res.Err(); err != nil {
			initErr = err
			return
		}
		
		nodeId := res.Val() % 1024 // Snowflake supports 0-1023 node IDs

		node, err := snowflake.NewNode(nodeId)
		if err != nil {
			initErr = err
			return
		}

		snowflakeNode = node
	})
	return initErr
}

// GenSnowflakeId 雪花算法生成ID
func GenSnowflakeId() int64 {
	if snowflakeNode == nil {
		panic("snowflake not initialized - call InitSnowflake first")
	}
	return snowflakeNode.Generate().Int64()
}
