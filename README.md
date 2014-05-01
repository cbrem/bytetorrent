ByteTorrent
===========

Overview
--------
  - A BitTorrent-like peer-to-peer application
  - Provides "client" and "tracker" implementations. The tracker mediates the exchange of file "chunks" by clients. "Torrents" associate file information with files on the tracker and client.
  - <code>src/torrent/</code>:
  Describe "Torrent" format. A torrent corresponds to exactly one data file, and contains information about that file:
      * the file's size
      * the hashes of the file's chunks
      * which trackers associate the file with this torrent
      * etc.
  - <code>src/tracker/</code>:
  Tracker is a cluster of nodes, replicated with Paxos. Tracker maintains a record of clients (identified by host:port) that claim to have chunks for torrents.
  - <code>src/client/</code>:
  Clients maintain a record of which files exist of a local file system, and which torrents correspond to these files. It can exchange chunks of these file with other clients.

Features
--------
  - Trackers use **Paxos** for replication. Paxos implementation is resiliant to failures in which a node crashes as well as failures in which a node stalleds indefinitely before resuming.
  - Trackers keep **logs** of committed operations in order to bring previously stalled nodes back up to speed.
  - Torrents are **identified by the hash of the associated data file**, to make name collisions almost impossible. In the event of a name collision, a tracker will not allow a torrent to be created.
  - Torrents contain information about the hashes of chunks of associated data files. Clients **check these hashes to ensure chunk integrity**.

Usage Example:
--------------
  1. Client A has a file <code>music.mp3</code> locally
  2. Client A CREATEs a torrent <code>music.torrent</code> for <code>music.mp3</code>
  3. Client A REGISTERs <code>music.torrent</code> with a tracker cluster. If it is successful, the tracker cluster will never associate other data file chunk hashes with this name.
  4. Client A OFFERs <code>music.mp3</code>, which informs the tracker that is has <code>music.torrent</code> and is ready to serve chunks.
  5. Client B finds <code>music.torrent</code> through conventional means.
  6. Client B DOWNLOADs <code>music.mp3</code> by contacting the trackers listed in <code>music.torrent</code>, getting a list of clients which contain <code>music.mp3</code> (possibly including Client A), and fetching chunks of <code>music.mp3</code> clients.

Tests
-----
  - <code>test/end\_to\_end/multi\_client\_real\_tracker\_test.sh</code>: 9 clients serve chunks of a data file to 1 client. A 3-node tracker cluster mediates.
  - <code>test/end\_to\_end/multi\_client\_real\_tracker\_malicious\_clients\_test.sh</code>: 4 honest clients and 5 malicious client serve chunks of a data file to 1 client. A 3-node tracker cluster mediates. The malicious nodes serve chunks with invalid hashes, and the received client must reject these chunks.
  - <code>test/end\_to\_end/multi\_client\_real\_tracker\_fail\_stop\_test.sh</code>: 9 clients serve chunks of a data file to 1 client. A 3-node tracker cluster mediates. One of the tracker nodes fails while the 9 nodes are informing the tracker cluster that they have chunks of the data file. This node does not recover.
  - <code>test/end\_to\_end/multi\_client\_real\_tracker\_fail\_stall\_test.sh</code>: 9 clients serve chunks of a data file to 1 client. A 3-node tracker cluster mediates. One of the tracker nodes goes offline while the 9 nodes are informing the tracker cluster that they have chunks of the data file. This node recovers after 5 seconds and is reintegrated into the cluster.

Progress Since Grading Meeting:
-------------------------------
  - Updated the testClosed and testClosedTwo functions in tests/trackertester/trackertester.go. The tracker now passes every time.
