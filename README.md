# raftd

Multi-group Raft as a daemon - build anything in any language on top of high performance multi-group Raft.

It manages all the complicated parts of raft like durable log management, log compaction, snapshotting and recovery, and 

<!-- TOC -->
* [raftd](#raftd)
  * [Integrating](#integrating)
    * [Running it](#running-it)
    * [Building the API](#building-the-api)
    * [Snapshots](#snapshots)
    * [Reading and writing via the raftd HTTP API](#reading-and-writing-via-the-raftd-http-api)
  * [Tips and Tricks](#tips-and-tricks)
    * [Use an HTTP/2 server](#use-an-http2-server)
    * [Keep Raft group data small](#keep-raft-group-data-small)
    * [Consider non-deterministic actions](#consider-non-deterministic-actions)
    * [Tuning snapshotting interval](#tuning-snapshotting-interval)
    * [Controlled SQLite WAL for instant snapshots](#controlled-sqlite-wal-for-instant-snapshots)
<!-- TOC -->

## Integrating

In order to integrate with raftd, you have to do three simple things:

1. Start running it alongside your application
2. Implement a handful of HTTP endpoints
3. Call to raftd's HTTP url to submit updates and linearizable reads

### Running it

### Building the API

Implementing the following endpoints is the most important and involved part of integration. But as you'll see, it's quite trivial to do.

All requests are POST requests.

- POST LastLogIndex - return the index of the last log entry that has been persisted
- POST UpdateEntries - update one or more entries in persistent storage (also storing the highest log index)
- POST Read - return data based on body provided body
- POST PrepareSnapshot - see Snapshots
- POST SaveSnapshot - see Snapshots
- POST RecoverFromSnapshot - see Snapshots

And you may optionally add the following endpoints
- POST Sync - This is an optimization RPC that is disabled by default. You can enable it with the `RAFT_SYNC=1` env var. Using this, it allows you to defer any fsync or batch commit calls (final durability) from UpdateEntries until this is called. If your UpdateEntries method already durably persists records to disk, then there is no need to use this. This is a pure optimization with some decently large complexity tradeoffs, so only use this if you know what you're doing. You can set `RAFT_SYNC=1` env var to use this. 

### Snapshots

Snapshots are only used when a new node joins the cluster, or a replica is sufficiently far behind that it cannot catch up purely via the log.

While `PrepareSnapshot` is called on a regular interval, `SaveSnapshot` is only called when data needs to be streamed to another machine, so it is acceptable to do this on-demand rather than preparing the whole snapshot and consuming significant extra disk.

Because the application code represents a Raft snapshot itself (a committed point in time that is compacted), automatic snapshotting only periodically calls `PrepareSnapshot`. This may be a file reference, a timestamp that can be used to create a full snapshot, etc. For example, you might return a timestamp that can be later used to generate a point-in-time KV backup ([e.g. the `since` value for badger](https://pkg.go.dev/github.com/dgraph-io/badger/v4#DB.Backup)). raftd will not persist the results of `PrepareSnapshot`, but rather uses a successful return as an indicator that it is safe to truncate the log up to this point in time.

**It's required that the results of `PrepareSnapshot` can be passed to `SaveSnapshot` to deterministically create snapshots.**

For example, if `SaveSnapshot` was called with the same parameters (return from `PrepareShapshot`) 1,000 times, each generated snapshot should be byte-for-byte the same. Multiple `PrepareSnapshot` values will not be stored, so you can assume that if `PrepareSnapshot` is called, the previous return value will never be used again.

`SaveSnapshot` generates and returns an actual snapshot that will be streamed to a remote replica. Because this file size may unknown at request time (e.g. you stream the backup creation directly to the HTTP response body), it is expected that this may be a streaming (HTTP2) or chunked (HTTP/1.1) response (no content-length response header). The raftd client will automatically handle this, so this should be used when possible to speed up snapshot creation.

`RecoverFromSnapshot` will receive a request where the body is the snapshot to restore from. This will have a known content-length, but could be quite a large request body. This should be streamed directly in for recovery as if you were reading a file from disk. raftd will download this snapshot to disk from the remote replica before making a request up to your application, so you never have to worry about partial snapshots due to network conditions being applied. See other tips and tricks for different scenarios in [Tips and Tricks](#tips-and-tricks).

It is recommended to leave automatic snapshotting enabled (which again, will only call `PrepareSnapshot`), with a reasonable snapshot frequency (e.g. every 1,000-10,000 updates, depending on update frequency, how large records are, and how resource-intensive a backup is).

**It is expected that snapshots can be created concurrently with other update operations.**

### Reading and writing via the raftd HTTP API

You may see the term "update" referred to in place of writes. Update is the Raft protocol-specific term used for mutating data. 

In order to write (update), it MUST go through Raft. And in order to do that, we must tell raftd that we are writing.

Even if your local instance is the leader and can process the write, it MUST submit it through raftd. raftd will call up to your application once it has reached consensus for that write to persist it.

## Tips and Tricks

### Use an HTTP/2 server

HTTP/2 (h2 or h2c) is wildly faster than HTTP/1.1. The raftd client automatically uses HTTP/2 when possible, including cleartext (http://).

### Keep Raft group data small

If you are going to have a massive database (100 GB+), you should be breaking it up into multiple Raft groups. For example, CockroachDB partitions after 512MB by default.

### Consider non-deterministic actions

If you have an instruction in the update record, for example `now()` in a SQL statement, consider that this will have different results when applied to different replicas, and thus break Raft's guarantees.

You should instead avoid non-deterministic actions in the update statements, and instead apply them before you save the data to Raft (e.g. generating the timestamp in your code, rather than in SQL).

### Tuning snapshotting interval

Tune this based on how large your updates are, how often you are inserting, and how resource intensive the snapshot preparation process is.

Every 1,000-10,000 logs is probably good.

### Controlled SQLite WAL for instant snapshots

Taking inspiration from [rqlite's new snapshotting approach](https://philipotoole.com/building-rqlite-9-0-cutting-disk-usage-by-half/#:~:text=New%20Snapshotting%20approach), if you are using SQLite as the storage engine in your application, we can modify the WAL checkpointing logic to make snapshotting instantaneous.

Go ahead and look at that post, but the gist of it is:
1. The DB file serves as the snapshot
2. The WAL only checkpoints when we call `PrepareSnapshot` (and never before)
3. `SaveSnapshot` can just return the DB file content (not the WAL)
4. `RecoverFromSnapshot` reads this from the request body, asks the remote node for the DB file, and streams it to local disk

It is wise to set automatic snapshotting on an interval (e.g. every 1,000-10,000 records depending on update frequency) to reduce how long recovery will still take.

This could be taken further by implementing a custom Raft log provider that uses the WAL as the log (using a custom WAL VFS), but that's not currently exposed (or suggested). However this could open it up to allowing for non-deterministic commands (e.g. `now()`) in SQL queries because the log would take data after the query executes, not before (e.g. using the SQL statements as the saved records).