package rediscluster

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/joomcode/redispipe/redis"
)

const testInfo = "# Server\r\nredis_version:7.2.4\r\navailability_zone:us-west-2a\r\n\r\n" +
	"# Persistence\r\nloading:0\r\n\r\n" +
	"# Replication\r\nrole:slave\r\nmaster_link_status:up\r\n"

func TestInfoField(t *testing.T) {
	info := []byte(testInfo)
	require.Equal(t, "us-west-2a", string(infoField(info, "availability_zone")))
	require.Equal(t, "up", string(infoField(info, "master_link_status")))
	require.Equal(t, "slave", string(infoField(info, "role")))
	require.Nil(t, infoField(info, "zone"))         // suffix of a field name is not a field
	require.Nil(t, infoField(info, "availability")) // prefix of a field name is not a field
	require.Nil(t, infoField(info, "missing"))
	require.Nil(t, infoField(nil, "availability_zone"))
	// empty value (zone is not configured on server)
	require.Equal(t, "", string(infoField([]byte("availability_zone:\r\n"), "availability_zone")))
	// no trailing newline
	require.Equal(t, "x", string(infoField([]byte("availability_zone:x"), "availability_zone")))
}

func TestSetReplicaInfo(t *testing.T) {
	sh := &shard{addr: []string{"m", "r1", "r2"}, good: 0b111, zone: "us-west-2a"}

	// healthy replica in client's zone
	sh.setReplicaInfo(redis.ByteResponse{Val: []byte(testInfo)}, 3)
	require.Equal(t, uint32(0b111), sh.good)
	require.Equal(t, uint32(0b010), sh.inZone)

	// healthy replica in other zone, plain []byte response
	sh.setReplicaInfo([]byte(strings.Replace(testInfo, "us-west-2a", "us-west-2b", 1)), 5)
	require.Equal(t, uint32(0b111), sh.good)
	require.Equal(t, uint32(0b010), sh.inZone)

	// replica lost its master: unhealthy, but zone is kept
	sh.setReplicaInfo([]byte(strings.Replace(testInfo, "master_link_status:up", "master_link_status:down", 1)), 3)
	require.Equal(t, uint32(0b101), sh.good)
	require.Equal(t, uint32(0b010), sh.inZone)

	// replica is loading
	sh.setReplicaInfo([]byte(strings.Replace(testInfo, "loading:0", "loading:1", 1)), 5)
	require.Equal(t, uint32(0b001), sh.good)

	// both recovered and are in client's zone
	sh.setReplicaInfo([]byte(testInfo), 3)
	sh.setReplicaInfo([]byte(testInfo), 5)
	require.Equal(t, uint32(0b111), sh.good)
	require.Equal(t, uint32(0b110), sh.inZone)

	// failed INFO clears both health and zone
	sh.setReplicaInfo(errors.New("ERR"), 3)
	require.Equal(t, uint32(0b101), sh.good)
	require.Equal(t, uint32(0b100), sh.inZone)

	// INFO without availability_zone (server without support) is healthy but not in zone
	sh.setReplicaInfo([]byte(strings.Replace(testInfo, "availability_zone:us-west-2a\r\n", "", 1)), 5)
	require.Equal(t, uint32(0b101), sh.good)
	require.Equal(t, uint32(0b000), sh.inZone)

	// READONLY response does not affect zone
	sh.setReplicaInfo([]byte(testInfo), 3)
	sh.setReplicaInfo(errors.New("ERR"), 2)
	require.Equal(t, uint32(0b101), sh.good)
	require.Equal(t, uint32(0b010), sh.inZone)

	// successful READONLY is a plain "OK" string: it must count as healthy, not as an unexpected response
	sh.setReplicaInfo("OK", 2)
	require.Equal(t, uint32(0b111), sh.good)
	require.Equal(t, uint32(0b010), sh.inZone)
}

func TestSetMasterInfo(t *testing.T) {
	sh := &shard{addr: []string{"m", "r1"}, good: 0b11, zone: "us-west-2a"}

	sh.setMasterInfo("OK", 0)
	sh.setMasterInfo([]byte("# Server\r\navailability_zone:us-west-2a\r\n"), 1)
	require.Equal(t, uint32(0b01), sh.inZone)
	require.Equal(t, uint32(0b11), sh.good)

	sh.setMasterInfo([]byte("# Server\r\navailability_zone:us-west-2b\r\n"), 1)
	require.Equal(t, uint32(0b00), sh.inZone)

	// master health is never touched by INFO
	sh.setMasterInfo([]byte(testInfo), 1)
	sh.setMasterInfo(errors.New("ERR"), 1)
	require.Equal(t, uint32(0b00), sh.inZone)
	require.Equal(t, uint32(0b11), sh.good)

	// without configured zone nothing is ever in zone
	sh = &shard{addr: []string{"m", "r1"}, good: 0b11}
	sh.setMasterInfo([]byte(testInfo), 1)
	sh.setReplicaInfo([]byte(testInfo), 3)
	require.Equal(t, uint32(0), sh.inZone)
	require.Equal(t, uint32(0b11), sh.good)
}
