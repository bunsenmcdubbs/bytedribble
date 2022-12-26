# bytedribble is a bad BitTorrent client

bytedribble seeks to implement an acceptable BitTorrent client with increasing correctness and completeness. This is 
purely a learning exercise and is currently (forever?) incomplete.

I am currently implementing the core BitTorrent protocol [BEP-3](https://www.bittorrent.org/beps/bep_0003.html) with 
the simple goal of downloading a file from a public tracker and participating in seeding as a well-behaved peer.
Afterwards, I'm interested in 
 - peer-to-peer extensions like the [DHT](https://www.bittorrent.org/beps/bep_0005.html) and
[PEX](https://www.bittorrent.org/beps/bep_0011.html) protocols
 - networking-specific enhancements with [uTP](https://www.bittorrent.org/beps/bep_0029.html) and support for
[holepunching](https://www.bittorrent.org/beps/bep_0055.html) (NAT traversal)
 - "completing" basic implementation with the piece selection algorithms outlined in the
[BitTorrent Economics paper](http://bittorrent.org/bittorrentecon.pdf) and 
[Fast Extension](https://www.bittorrent.org/beps/bep_0006.html)
 - extending the interface and design to be server-focused
 - implementing a tracker
 - rust rewrite

In addition to unit tests, I use a "lab" of docker containers to test end-to-end functionality by running a simple
tracker and other (functional) BitTorrent client/peers (Transmission).