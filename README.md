# raftd

Multi-group Raft as a daemon - build anything in any language on top of high performance multi-group Raft.

It manages all of the complicated parts of raft like durable log management, log compaction, snapshotting and recovery, and 

## Integrating

In order to integrate with raftd, you have to do three simple things:

1. Start running it alongside your application
2. Implement a handful of HTTP endpoints
3. Call to raftd's HTTP url to submit updates and linearizable reads

### Running it

### Building the API

Implementing the following endpoints is the most important and involved part of integration. But as you'll see, it's quite trivial to do.

- GET LastLogIndex - return the index of the last log entry that has been persisted
- POST UpdateEntries - update one or more entries in persistent storage (also storing the highest log index)
- GET GetEntry - return a given entry by some identifier, the body will be the body that was submitted to raftd for a linearizable read
- POST PrepareSnapshot - see Snapshots
- POST SaveSnapshot - see Snapshots
- POST RecoverFromSnapshot - see Snapshots
- 

And you may optionally add the following endpoints
- POST Sync - This is an optimization RPC that is disabled by default. You can enable it with the `RAFT_SYNC=1` env var. Using this, it allows you to defer any fsync or batch commit calls (final durability) from UpdateEntries until this is called. If your UpdateEntries method already durably persists records to disk, then there is no need to use this. This is a pure optimization with some decently large complexity tradeoffs, so only use this if you know what you're doing

### Snapshots

Snapshots are only used when a new node joins the cluster, or a replica is sufficiently far behind that it cannot catch up purely via the log.

PrepareSnapshot simply returns an in-memory record for a point in time at which a snapshot can be saved. For example it might call `fork`, or simply return `{}` if your database can already perform backups concurrently with processing other operations. This should do no work in terms of actually creating serializing a snapshot, just prepare the system for it.

SaveSnapshot will actually return a stream of data that is the snapshot. Because this file size may not be known at request time (e.g. you stream the backup creation directly to the HTTP response body), it is expected that this will be a streaming (HTTP2) or chunked (HTTP/1.1) response (no content-length response header). The raftd client will automatically handle this, so this should be used when possible to speed up snapshot creation.

RecoverFromSnapshots will receive a request where the body is the snapshot to restore from. This will have a known content-length, but could be quite a large request body. If possible, this should be streamed directly in for recovery. See other tips and tricks for different scenarios in Tips and Tricks

### Reading and writing via the raftd HTTP API

You may see the term "update" referred to in place of writes. Update is the Raft protocol-specific term used for mutating data. 

In order to write (update), it MUST go through Raft. And in order to do that, we must tell raftd that we are writing.

Even if your local instance is the leader and can process the write, it MUST submit it through raftd. raftd will call up to your application once it has reached consensus for that write to persist it.

## Tips and Tricks

### Controlled SQLite WAL for instant snapshots - WIP

WIP https://github.com/lni/dragonboat/issues/375

Taking inspiration from [rqlite's new snapshotting approach](https://philipotoole.com/building-rqlite-9-0-cutting-disk-usage-by-half/#:~:text=New%20Snapshotting%20approach), if you are using SQLite as the storage engine in your application, we can modify the WAL checkpointing logic to make snapshotting instantaneous.

Go ahead and look at that post, but the gist of it is:
1. The DB file serves as the snapshot
2. The WAL only checkpoints when we create a snaphot (and never before)
3. Snapshotting can just return a reference to the WAL, rather than