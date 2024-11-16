# CadenceDB
An implementation of a caching database in Go. This supports replication with master-replica nodes.

### Technical Specs:
- Server is a TCP server.
- Serialization protocol is a variant of the Redis Serialization Protocol (RESP).
  
### How to use:
To run a node, just run the following command:
`go run ./src`
You can also add the following flags while running the command:
- `--port="[port]"`: sets the port on which to run the TCP server (by default it is 6379, the default port for Redis servers).
- `--replicaof="[hostAddress hostPort]"`: tells the node which node it is a replica of.

Currently it can be interacted with the `redis-cli` (working on making a custom cli for this specifically), and supports the following commands:
- `PING`: simple status check (should reply with "PONG" if node is alive)
- `SET [key] [value]`: add a key value pair to the cache; optionally add "PX [expiryTimeInMilliSec]"
- `GET [key]`: get the value for a particular key - if key doesn't exist, returns the `nil` string
- `ECHO [string]`: echoes a message
- `INFO`: get info about the node (whether its a replica or not, how many bytes its processed so far)

### Future Plans (currently in progress)
Add:
- Horizontal sharding
- Load balancing for reads across replica nodes.
- Fault tolerance (automatic failover handling, like electing new primary node in case it fails)
- Persistence (logging or something like RDB)
- A client library to easily integrate with Node.js projects

Try to:
- turn this into a primary NoSQL database (supporting efficient indexing and range queries)
- turn this into a real-time database (like Firebase)
