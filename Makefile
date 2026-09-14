VALKEY_ARCHIVE ?= https://github.com/valkey-io/valkey/archive
VALKEY_VERSION ?= 8.1.10

test: testcluster testconn testredis

/tmp/valkey-server/valkey-server:
	@echo "Building valkey-$(VALKEY_VERSION)..."
	wget -nv -c $(VALKEY_ARCHIVE)/$(VALKEY_VERSION).tar.gz -O - | tar -xzC .
	cd valkey-$(VALKEY_VERSION) && make -j 4 USE_JEMALLOC=no
	if [ ! -e /tmp/valkey-server ] ; then mkdir /tmp/valkey-server ; fi
	mv valkey-$(VALKEY_VERSION)/src/valkey-server /tmp/valkey-server
	rm valkey-$(VALKEY_VERSION) -rf

testredis:
	PATH=/tmp/valkey-server/:${PATH} go test ./redis

testconn: /tmp/valkey-server/valkey-server
	killall valkey-server || true
	rm ./redisconn/redis_test_* -r || true
	PATH=/tmp/valkey-server/:${PATH} go test -count 1 ./redisconn

testcluster: /tmp/valkey-server/valkey-server
	killall valkey-server || true
	rm ./rediscluster/redis_test_* -r || true
	PATH=/tmp/valkey-server/:${PATH} go test -count 1 -tags debugredis ./rediscluster

bench: benchconn benchcluster

benchconn: /tmp/valkey-server/valkey-server
	PATH=/tmp/valkey-server/:${PATH} ; cd ./redisconn/bench ; go test -count 1 -run FooBar -bench . -benchmem .

benchcluster: /tmp/valkey-server/valkey-server
	PATH=/tmp/valkey-server/:${PATH} ; cd ./rediscluster/bench ; go test -count 1 -tags debugredis -run FooBar -bench . -benchmem .

clean:
	rm -r */redis_test_*
