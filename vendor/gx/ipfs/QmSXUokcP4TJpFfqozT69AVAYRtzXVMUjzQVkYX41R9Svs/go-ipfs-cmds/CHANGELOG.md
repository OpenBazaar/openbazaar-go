# go-ipfs changelog

### 0.4.4 - 2016-10-11

This release contains an important hotfix for a bug we discovered in how pinning works.
If you had a large number of pins, new pins would overwrite existing pins.
Apart from the hotfix, this release is equal to the previous release 0.4.3.

- Fix bug in pinsets fanout, and add stress test. (@whyrusleeping, [ipfs/go-ipfs#3273](https://github.com/ipfs/go-ipfs/pull/3273))

We published a [detailed account of the bug and fix in a blog post](https://ipfs.io/blog/21-go-ipfs-0-4-4-released/).

### 0.4.3 - 2016-09-20

There have been no changes since the last release candidate 0.4.3-rc4. \o/

### 0.4.3-rc4 - 2016-09-09

This release candidate fixes issues in Bitswap and the `ipfs add` command, and improves testing.
We plan for this to be the last release candidate before the release of go-ipfs v0.4.3.

With this release candidate, we're also moving go-ipfs to Go 1.7, which we expect will yield improvements in runtime performance, memory usage, build time and size of the release binaries.

- Require Go 1.7. (@whyrusleeping, @Kubuxu, @lgierth, [ipfs/go-ipfs#3163](https://github.com/ipfs/go-ipfs/pull/3163))
  - For this purpose, switch Docker image from Alpine 3.4 to Alpine Edge.
- Fix cancellation of Bitswap `wantlist` entries. (@whyrusleeping, [ipfs/go-ipfs#3182](https://github.com/ipfs/go-ipfs/pull/3182))
- Fix clearing of `active` state of Bitswap provider queries. (@whyrusleeping, [ipfs/go-ipfs#3169](https://github.com/ipfs/go-ipfs/pull/3169))
- Fix a panic in the DHT code. (@Kubuxu, [ipfs/go-ipfs#3200](https://github.com/ipfs/go-ipfs/pull/3200))
- Improve handling of `Identity` field in `ipfs config` command. (@Kubuxu, @whyrusleeping, [ipfs/go-ipfs#3141](https://github.com/ipfs/go-ipfs/pull/3141))
- Fix explicit adding of symlinked files and directories. (@kevina, [ipfs/go-ipfs#3135](https://github.com/ipfs/go-ipfs/pull/3135))
- Fix bash auto-completion of `ipfs daemon --unrestricted-api` option. (@lgierth, [ipfs/go-ipfs#3159](https://github.com/ipfs/go-ipfs/pull/3159))
- Introduce a new timeout tool for tests to avoid licensing issues. (@Kubuxu, [ipfs/go-ipfs#3152](https://github.com/ipfs/go-ipfs/pull/3152))
- Improve output for migrations of fs-repo. (@lgierth, [ipfs/go-ipfs#3158](https://github.com/ipfs/go-ipfs/pull/3158))
- Fix info notice of commands taking input from stdin. (@Kubuxu, [ipfs/go-ipfs#3134](https://github.com/ipfs/go-ipfs/pull/3134))
- Bring back a few tests for stdin handling of `ipfs cat` and `ipfs add`. (@Kubuxu, [ipfs/go-ipfs#3144](https://github.com/ipfs/go-ipfs/pull/3144))
- Improve sharness tests for `ipfs repo verify` command. (@whyrusleeping, [ipfs/go-ipfs#3148](https://github.com/ipfs/go-ipfs/pull/3148))
- Improve sharness tests for CORS headers on the gateway. (@Kubuxu, [ipfs/go-ipfs#3142](https://github.com/ipfs/go-ipfs/pull/3142))
- Improve tests for pinning within `ipfs files`. (@kevina, [ipfs/go-ipfs#3151](https://github.com/ipfs/go-ipfs/pull/3151))
- Improve tests for the automatic raising of file descriptor limits. (@whyrusleeping, [ipfs/go-ipfs#3149](https://github.com/ipfs/go-ipfs/pull/3149))

### 0.4.3-rc3 - 2016-08-11

This release candidate fixes a panic that occurs when input from stdin was
expected, but none was given: [ipfs/go-ipfs#3050](https://github.com/ipfs/go-ipfs/pull/3050)

### 0.4.3-rc2 - 2016-08-04

This release includes bugfixes and fixes for regressions that were introduced
between 0.4.2 and 0.4.3-rc1.

- Regressions
  - Fix daemon panic when there is no multipart input provided over the HTTP API.
  (@whyrusleeping, [ipfs/go-ipfs#2989](https://github.com/ipfs/go-ipfs/pull/2989))
  - Fix `ipfs refs --edges` not printing edges.
  (@Kubuxu, [ipfs/go-ipfs#3007](https://github.com/ipfs/go-ipfs/pull/3007))
  - Fix progress option for `ipfs add` defaulting to true on the HTTP API.
  (@whyrusleeping, [ipfs/go-ipfs#3025](https://github.com/ipfs/go-ipfs/pull/3025))
  - Fix erroneous printing of stdin reading message.
  (@whyrusleeping, [ipfs/go-ipfs#3033](https://github.com/ipfs/go-ipfs/pull/3033))
  - Fix panic caused by passing `--mount` and `--offline` flags to `ipfs daemon`.
  (@Kubuxu, [ipfs/go-ipfs#3022](https://github.com/ipfs/go-ipfs/pull/3022))
  - Fix symlink path resolution on windows.
  (@Kubuxu, [ipfs/go-ipfs#3023](https://github.com/ipfs/go-ipfs/pull/3023))
  - Add in code to prevent issue 3032 from crashing the daemon.
  (@whyrusleeping, [ipfs/go-ipfs#3037](https://github.com/ipfs/go-ipfs/pull/3037))


### 0.4.3-rc1 - 2016-07-23

This is a maintenance release which comes with a couple of nice enhancements, and improves the performance of Storage, Bitswap, as well as Content and Peer Routing. It also introduces a handful of new commands and options, and fixes a good bunch of bugs.

This is the first Release Candidate. Unless there are vulnerabilities or regressions discovered, the final 0.4.3 release will happen about one week from now.

- Security Vulnerability

  - The `master` branch if go-ipfs suffered from a vulnerability for about 3 weeks. It allowed an attacker to use an iframe to request malicious HTML and JS from the API of a local go-ipfs node. The attacker could then gain unrestricted access to the node's API, and e.g. extract the private key. We fixed this issue by reintroducing restrictions on which particular objects can be loaded through the API (@lgierth, [ipfs/go-ipfs#2949](https://github.com/ipfs/go-ipfs/pull/2949)), and by completely excluding the private key from the API (@Kubuxu, [ipfs/go-ipfs#2957](https://github.com/ipfs/go-ipfs/pull/2957)). We will also work on more hardening of the API in the next release.
  - **The previous release 0.4.2 is not vulnerable. That means if you're using official binaries from [dist.ipfs.io](https://dist.ipfs.io) you're not affected.** If you're running go-ipfs built from the `master` branch between June 17th ([ipfs/go-ipfs@1afebc21](https://github.com/ipfs/go-ipfs/commit/1afebc21f324982141ca8a29710da0d6f83ca804)) and July 7th ([ipfs/go-ipfs@39bef0d5](https://github.com/ipfs/go-ipfs/commit/39bef0d5b01f70abf679fca2c4d078a2d55620e2)), please update to v0.4.3-rc1 immediately.
  - We are grateful to the group of independent researchers who made us aware of this vulnerability. We wanna use this opportunity to reiterate that we're very happy about any additional review of pull requests and releases. You can contact us any time at security@ipfs.io (GPG [4B9665FB 92636D17 7C7A86D3 50AAE8A9 59B13AF3](https://pgp.mit.edu/pks/lookup?op=get&search=0x50AAE8A959B13AF3)).

- Notable changes

  - Improve Bitswap performance. (@whyrusleeping, [ipfs/go-ipfs#2727](https://github.com/ipfs/go-ipfs/pull/2727), [ipfs/go-ipfs#2798](https://github.com/ipfs/go-ipfs/pull/2798))
  - Improve Content Routing and Peer Routing performance. (@whyrusleeping, [ipfs/go-ipfs#2817](https://github.com/ipfs/go-ipfs/pull/2817), [ipfs/go-ipfs#2841](https://github.com/ipfs/go-ipfs/pull/2841))
  - Improve datastore, blockstore, and dagstore performance. (@kevina, @Kubuxu, @whyrusleeping [ipfs/go-datastore#43](https://github.com/ipfs/go-datastore/pull/43), [ipfs/go-ipfs#2885](https://github.com/ipfs/go-ipfs/pull/2885), [ipfs/go-ipfs#2961](https://github.com/ipfs/go-ipfs/pull/2961), [ipfs/go-ipfs#2953](https://github.com/ipfs/go-ipfs/pull/2953), [ipfs/go-ipfs#2960](https://github.com/ipfs/go-ipfs/pull/2960))
  - Content Providers are now stored on disk to gain savings on process memory. (@whyrusleeping, [ipfs/go-ipfs#2804](https://github.com/ipfs/go-ipfs/pull/2804), [ipfs/go-ipfs#2860](https://github.com/ipfs/go-ipfs/pull/2860))
  - Migrations of the fs-repo (usually stored at `~/.ipfs`) now run automatically. If there's a TTY available, you'll get prompted when running `ipfs daemon`, and in addition you can use the `--migrate=true` or `--migrate=false` options to avoid the prompt. (@whyrusleeping, @lgierth, [ipfs/go-ipfs#2939](https://github.com/ipfs/go-ipfs/pull/2939))
  - The internal naming of blocks in the blockstore has changed, which requires a migration of the fs-repo, from version 3 to 4. (@whyrusleeping, [ipfs/go-ipfs#2903](https://github.com/ipfs/go-ipfs/pull/2903))
  - We now automatically raise the file descriptor limit to 1024 if neccessary. (@whyrusleeping, [ipfs/go-ipfs#2884](https://github.com/ipfs/go-ipfs/pull/2884), [ipfs/go-ipfs#2891](https://github.com/ipfs/go-ipfs/pull/2891))
  - After a long struggle with deadlocks and hanging connections, we've decided to disable the uTP transport by default for now. (@whyrusleeping, [ipfs/go-ipfs#2840](https://github.com/ipfs/go-ipfs/pull/2840), [ipfs/go-libp2p-transport@88244000](https://github.com/ipfs/go-libp2p-transport/commit/88244000f0ce8851ffcfbac746ebc0794b71d2a4))
  - There is now documentation for the configuration options in `docs/config.md`. (@whyrusleeping, [ipfs/go-ipfs#2974](https://github.com/ipfs/go-ipfs/pull/2974))
  - All commands now sanely handle the combination of stdin and optional flags in certain edge cases. (@lgierth, [ipfs/go-ipfs#2952](https://github.com/ipfs/go-ipfs/pull/2952))

- New Features

  - Add `--offline` option to `ipfs daemon` command, which disables all swarm networking. (@Kubuxu, [ipfs/go-ipfs#2696](https://github.com/ipfs/go-ipfs/pull/2696), [ipfs/go-ipfs#2867](https://github.com/ipfs/go-ipfs/pull/2867))
  - Add `Datastore.HashOnRead` option for verifying block hashes on read access. (@Kubuxu, [ipfs/go-ipfs#2904](https://github.com/ipfs/go-ipfs/pull/2904))
  - Add `Datastore.BloomFilterSize` option for tuning the blockstore's new lookup bloom filter. (@Kubuxu, [ipfs/go-ipfs#2973](https://github.com/ipfs/go-ipfs/pull/2973))

- Bugfixes

  - Fix publishing of local IPNS entries, and more. (@whyrusleeping, [ipfs/go-ipfs#2943](https://github.com/ipfs/go-ipfs/pull/2943))
  - Fix progress bars in `ipfs add` and `ipfs get`. (@whyrusleeping, [ipfs/go-ipfs#2893](https://github.com/ipfs/go-ipfs/pull/2893), [ipfs/go-ipfs#2948](https://github.com/ipfs/go-ipfs/pull/2948))
  - Make sure files added through `ipfs files` are pinned and don't get GC'd. (@kevina, [ipfs/go-ipfs#2872](https://github.com/ipfs/go-ipfs/pull/2872))
  - Fix copying into directory using `ipfs files cp`. (@whyrusleeping, [ipfs/go-ipfs#2977](https://github.com/ipfs/go-ipfs/pull/2977))
  - Fix `ipfs version --commit` with Docker containers. (@lgierth, [ipfs/go-ipfs#2734](https://github.com/ipfs/go-ipfs/pull/2734))
  - Run `ipfs diag` commands in the daemon instead of the CLI. (@Kubuxu, [ipfs/go-ipfs#2761](https://github.com/ipfs/go-ipfs/pull/2761))
  - Fix protobuf encoding on the API and in commands. (@stebalien, [ipfs/go-ipfs#2516](https://github.com/ipfs/go-ipfs/pull/2516))
  - Fix goroutine leak in `/ipfs/ping` protocol handler. (@whyrusleeping, [ipfs/go-libp2p#58](https://github.com/ipfs/go-libp2p/pull/58))
  - Fix `--flags` option on `ipfs commands`. (@Kubuxu, [ipfs/go-ipfs#2773](https://github.com/ipfs/go-ipfs/pull/2773))
  - Fix the error channels in `namesys`. (@whyrusleeping, [ipfs/go-ipfs#2788](https://github.com/ipfs/go-ipfs/pull/2788))
  - Fix consumptions of observed swarm addresses. (@whyrusleeping, [ipfs/go-libp2p#63](https://github.com/ipfs/go-libp2p/pull/63), [ipfs/go-ipfs#2771](https://github.com/ipfs/go-ipfs/issues/2771))
  - Fix a rare DHT panic. (@whyrusleeping, [ipfs/go-ipfs#2856](https://github.com/ipfs/go-ipfs/pull/2856))
  - Fix go-ipfs/js-ipfs interoperability issues in SPDY. (@whyrusleeping, [whyrusleeping/go-smux-spdystream@fae17783](https://github.com/whyrusleeping/go-smux-spdystream/commit/fae1778302a9e029bb308cf71cf33f857f2d89e8))
  - Fix a logging race condition during shutdown. (@Kubuxu, [ipfs/go-log#3](https://github.com/ipfs/go-log/pull/3))
  - Prevent DHT connection hangs. (@whyrusleeping, [ipfs/go-ipfs#2826](https://github.com/ipfs/go-ipfs/pull/2826), [ipfs/go-ipfs#2863](https://github.com/ipfs/go-ipfs/pull/2863))
  - Fix NDJSON output of `ipfs refs local`. (@Kubuxu, [ipfs/go-ipfs#2812](https://github.com/ipfs/go-ipfs/pull/2812))
  - Fix race condition in NAT detection. (@whyrusleeping, [ipfs/go-libp2p#69](https://github.com/ipfs/go-libp2p/pull/69))
  - Fix error messages. (@whyrusleeping, @Kubuxu, [ipfs/go-ipfs#2905](https://github.com/ipfs/go-ipfs/pull/2905), [ipfs/go-ipfs#2928](https://github.com/ipfs/go-ipfs/pull/2928))

- Enhancements

  - Increase maximum object size on `ipfs put` from 1 MiB to 2 MiB. The maximum object size on the wire including all framing is 4 MiB. (@kpcyrd, [ipfs/go-ipfs#2980](https://github.com/ipfs/go-ipfs/pull/2980))
  - Add CORS headers to the Gateway's default config. (@Kubuxu, [ipfs/go-ipfs#2778](https://github.com/ipfs/go-ipfs/pull/2778))
  - Clear the dial backoff for a peer when using `ipfs swarm connect`. (@whyrusleeping, [ipfs/go-ipfs#2941](https://github.com/ipfs/go-ipfs/pull/2941))
  - Allow passing options to daemon in Docker container. (@lgierth, [ipfs/go-ipfs#2955](https://github.com/ipfs/go-ipfs/pull/2955))
  - Add `-v/--verbose` to `ìpfs swarm peers` command. (@csasarak, [ipfs/go-ipfs#2713](https://github.com/ipfs/go-ipfs/pull/2713))
  - Add `--format`, `--hash`, and `--size` options to `ipfs files stat` command. (@Kubuxu, [ipfs/go-ipfs#2706](https://github.com/ipfs/go-ipfs/pull/2706))
  - Add `--all` option to `ipfs version` command. (@Kubuxu, [ipfs/go-ipfs#2790](https://github.com/ipfs/go-ipfs/pull/2790))
  - Add `ipfs repo version` command. (@pfista, [ipfs/go-ipfs#2598](https://github.com/ipfs/go-ipfs/pull/2598))
  - Add `ipfs repo verify` command. (@whyrusleeping, [ipfs/go-ipfs#2924](https://github.com/ipfs/go-ipfs/pull/2924), [ipfs/go-ipfs#2951](https://github.com/ipfs/go-ipfs/pull/2951))
  - Add `ipfs stats repo` and `ipfs stats bitswap` command aliases. (@pfista, [ipfs/go-ipfs#2810](https://github.com/ipfs/go-ipfs/pull/2810))
  - Add success indication to responses of `ipfs ping` command. (@Kubuxu, [ipfs/go-ipfs#2813](https://github.com/ipfs/go-ipfs/pull/2813))
  - Save changes made via `ipfs swarm filter` to the config file. (@yuvallanger, [ipfs/go-ipfs#2880](https://github.com/ipfs/go-ipfs/pull/2880))
  - Expand `ipfs_p2p_peers` metric to include libp2p transport. (@lgierth, [ipfs/go-ipfs#2728](https://github.com/ipfs/go-ipfs/pull/2728))
  - Rework `ipfs files add` internals to avoid caching and prevent memory leaks. (@whyrusleeping, [ipfs/go-ipfs#2795](https://github.com/ipfs/go-ipfs/pull/2795))
  - Support `GOPATH` with multiple path components. (@karalabe, @lgierth, @djdv, [ipfs/go-ipfs#2808](https://github.com/ipfs/go-ipfs/pull/2808), [ipfs/go-ipfs#2862](https://github.com/ipfs/go-ipfs/pull/2862), [ipfs/go-ipfs#2975](https://github.com/ipfs/go-ipfs/pull/2975))

- General Codebase

  - Take steps towards the `filestore` datastore. (@kevina, [ipfs/go-ipfs#2792](https://github.com/ipfs/go-ipfs/pull/2792), [ipfs/go-ipfs#2634](https://github.com/ipfs/go-ipfs/pull/2634))
  - Update recommended Golang version to 1.6.2 (@Kubuxu, [ipfs/go-ipfs#2724](https://github.com/ipfs/go-ipfs/pull/2724))
  - Update to Gx 0.8.0 and Gx-Go 1.2.1, which is faster and less noisy. (@whyrusleeping, [ipfs/go-ipfs#2979](https://github.com/ipfs/go-ipfs/pull/2979))
  - Use `go4.org/lock` instead of `camlistore/lock` for locking. (@whyrusleeping, [ipfs/go-ipfs#2887](https://github.com/ipfs/go-ipfs/pull/2887))
  - Manage `go.uuid`, `hamming`, `backoff`, `proquint`, `pb`, `go-context`, `cors`, `go-datastore` packages with Gx. (@Kubuxu, [ipfs/go-ipfs#2733](https://github.com/ipfs/go-ipfs/pull/2733), [ipfs/go-ipfs#2736](https://github.com/ipfs/go-ipfs/pull/2736), [ipfs/go-ipfs#2757](https://github.com/ipfs/go-ipfs/pull/2757), [ipfs/go-ipfs#2825](https://github.com/ipfs/go-ipfs/pull/2825), [ipfs/go-ipfs#2838](https://github.com/ipfs/go-ipfs/pull/2838))
  - Clean up the gateway's surface. (@lgierth, [ipfs/go-ipfs#2874](https://github.com/ipfs/go-ipfs/pull/2874))
  - Simplify the API gateway's access restrictions. (@lgierth, [ipfs/go-ipfs#2949](https://github.com/ipfs/go-ipfs/pull/2949), [ipfs/go-ipfs#2956](https://github.com/ipfs/go-ipfs/pull/2956))
  - Update docker image to Alpine Linux 3.4 and remove Go version constraint. (@lgierth, [ipfs/go-ipfs#2901](https://github.com/ipfs/go-ipfs/pull/2901), [ipfs/go-ipfs#2929](https://github.com/ipfs/go-ipfs/pull/2929))
  - Clarify `Dockerfile` and `Dockerfile.fast`. (@lgierth, [ipfs/go-ipfs#2796](https://github.com/ipfs/go-ipfs/pull/2796))
  - Simplify resolution of Git commit refs in Dockerfiles. (@lgierth, [ipfs/go-ipfs#2754](https://github.com/ipfs/go-ipfs/pull/2754))
  - Consolidate `--verbose` description across commands. (@Kubuxu, [ipfs/go-ipfs#2746](https://github.com/ipfs/go-ipfs/pull/2746))
  - Allow setting position of default values in command option descriptions. (@Kubuxu, [ipfs/go-ipfs#2744](https://github.com/ipfs/go-ipfs/pull/2744))
  - Set explicit default values for boolean command options. (@RichardLitt, [ipfs/go-ipfs#2657](https://github.com/ipfs/go-ipfs/pull/2657))
  - Autogenerate command synopsises. (@Kubuxu, [ipfs/go-ipfs#2785](https://github.com/ipfs/go-ipfs/pull/2785))
  - Fix and improve lots of documentation. (@RichardLitt, [ipfs/go-ipfs#2741](https://github.com/ipfs/go-ipfs/pull/2741), [ipfs/go-ipfs#2781](https://github.com/ipfs/go-ipfs/pull/2781))
  - Improve command descriptions to fit a width of 78 characters. (@RichardLitt, [ipfs/go-ipfs#2779](https://github.com/ipfs/go-ipfs/pull/2779), [ipfs/go-ipfs#2780](https://github.com/ipfs/go-ipfs/pull/2780), [ipfs/go-ipfs#2782](https://github.com/ipfs/go-ipfs/pull/2782))
  - Fix filename conflict in the debugging guide. (@Kubuxu, [ipfs/go-ipfs#2752](https://github.com/ipfs/go-ipfs/pull/2752))
  - Decapitalize log messages, according to Golang style guides. (@RichardLitt, [ipfs/go-ipfs#2853](https://github.com/ipfs/go-ipfs/pull/2853))
  - Add Github Issues HowTo guide. (@RichardLitt, @chriscool, [ipfs/go-ipfs#2889](https://github.com/ipfs/go-ipfs/pull/2889), [ipfs/go-ipfs#2895](https://github.com/ipfs/go-ipfs/pull/2895))
  - Add Github Issue template. (@chriscool, [ipfs/go-ipfs#2786](https://github.com/ipfs/go-ipfs/pull/2786))
  - Apply standard-readme to the README file. (@RichardLitt, [ipfs/go-ipfs#2883](https://github.com/ipfs/go-ipfs/pull/2883))
  - Fix issues pointed out by `govet`. (@Kubuxu, [ipfs/go-ipfs#2854](https://github.com/ipfs/go-ipfs/pull/2854))
  - Clarify `ipfs get` error message. (@whyrusleeping, [ipfs/go-ipfs#2886](https://github.com/ipfs/go-ipfs/pull/2886))
  - Remove dead code. (@whyrusleeping, [ipfs/go-ipfs#2819](https://github.com/ipfs/go-ipfs/pull/2819))
  - Add changelog for v0.4.3. (@lgierth, [ipfs/go-ipfs#2984](https://github.com/ipfs/go-ipfs/pull/2984))

- Tests & CI

  - Fix flaky `ipfs mount` sharness test by using the `iptb` tool. (@noffle, [ipfs/go-ipfs#2707](https://github.com/ipfs/go-ipfs/pull/2707))
  - Fix flaky IP port selection in tests. (@Kubuxu, [ipfs/go-ipfs#2855](https://github.com/ipfs/go-ipfs/pull/2855))
  - Fix CLI tests on OSX by resolving /tmp symlink. (@Kubuxu, [ipfs/go-ipfs#2926](https://github.com/ipfs/go-ipfs/pull/2926))
  - Fix flaky GC test by running the daemon in offline mode. (@Kubuxu, [ipfs/go-ipfs#2908](https://github.com/ipfs/go-ipfs/pull/2908))
  - Add tests for `ipfs add` with hidden files. (@Kubuxu, [ipfs/go-ipfs#2756](https://github.com/ipfs/go-ipfs/pull/2756))
  - Add test to make sure the body of HEAD responses is empty. (@Kubuxu, [ipfs/go-ipfs#2775](https://github.com/ipfs/go-ipfs/pull/2775))
  - Add test to catch misdials. (@Kubuxu, [ipfs/go-ipfs#2831](https://github.com/ipfs/go-ipfs/pull/2831))
  - Mark flaky tests for `ipfs dht query` as known failure. (@noffle, [ipfs/go-ipfs#2720](https://github.com/ipfs/go-ipfs/pull/2720))
  - Remove failing blockstore-without-context test. (@Kubuxu, [ipfs/go-ipfs#2857](https://github.com/ipfs/go-ipfs/pull/2857))
  - Fix `--version` tests for versions with a suffix like `-dev` or `-rc1`. (@lgierth, [ipfs/go-ipfs#2937](https://github.com/ipfs/go-ipfs/pull/2937))
  - Make sharness tests work in cases where go-ipfs is symlinked into GOPATH. (@lgierth, [ipfs/go-ipfs#2937](https://github.com/ipfs/go-ipfs/pull/2937))
  - Add variable delays to blockstore mocks. (@rikonor, [ipfs/go-ipfs#2871](https://github.com/ipfs/go-ipfs/pull/2871))
  - Disable Travis CI email notifications. (@Kubuxu, [ipfs/go-ipfs#2896](https://github.com/ipfs/go-ipfs/pull/2896))


### 0.4.2 - 2016-05-17

This is a patch release which fixes perfomance and networking bugs in go-libp2p,
You should see improvements in CPU and RAM usage, as well as speed of object lookups.
There are also a few other nice improvements.

* Notable Fixes
  * Set a deadline for dialing attempts. This prevents a node from accumulating
    failed connections. (@whyrusleeping)
  * Avoid unneccessary string/byte conversions in go-multihash. (@whyrusleeping)
  * Fix a deadlock around the yamux stream muxer. (@whyrusleeping)
  * Fix a bug that left channels open, causing hangs. (@whyrusleeping)
  * Fix a bug around yamux which caused connection hangs. (@whyrusleeping)
  * Fix a crash caused by nil multiaddrs. (@whyrusleeping)

* Enhancements
  * Add NetBSD support. (@erde74)
  * Set Cache-Control: immutable on /ipfs responses. (@kpcyrd)
  * Have `ipfs init` optionally accept a default configuration from stdin. (@sivachandran)
  * Add `ipfs log ls` command for listing logging subsystems. (@hsanjuan)
  * Allow bitswap to read multiple messages per stream. (@whyrusleeping)
  * Remove `make toolkit_upgrade` step. (@chriscool)

* Documentation
  * Add a debug-guidelines document. (@richardlitt)
  * Update the contribute document. (@richardlitt)
  * Fix documentation of many `ipfs` commands. (@richardlitt)
  * Fall back to ShortDesc if LongDesc is missing. (@Kubuxu)

* Removals
  * Remove -f option from `ipfs init` command. (@whyrusleeping)

* Bugfixes
  * Fix `ipfs object patch` argument handling and validation. (@jbenet)
  * Fix `ipfs config edit` command by running it client-side. (@Kubuxu)
  * Set default value for `ipfs refs` arguments. (@richardlitt)
  * Fix parsing of incorrect command and argument permutations. (@thomas-gardner)
  * Update Dockerfile to latest go1.5.4-r0. (@chriscool)
  * Allow passing IPFS_LOGGING to Docker image. (@lgierth)
  * Fix dot path parsing on Windows. (@djdv)
  * Fix formatting of `ipfs log ls` output. (@richardlitt)

* General Codebase
  * Refactor Makefile. (@kevina)
  * Wire context into bitswap requests more deeply. (@whyrusleeping)
  * Use gx for iptb. (@chriscool)
  * Update gx and gx-go. (@chriscool)
  * Make blocks.Block an interface. (@kevina)
  * Silence check for Docker existance. (@chriscool)
  * Add dist_get script for fetching tools from dist.ipfs.io. (@whyrusleeping)
  * Add proper defaults to all `ipfs` commands. (@richardlitt)
  * Remove dead `count` option from `ipfs pin ls`. (@richardlitt)
  * Initialize pin mode strings only once. (@chriscool)
  * Add changelog for v0.4.2. (@lgierth)
  * Specify a dist.ipfs.io hash for tool downloads instead of trusting DNS. (@lgierth)

* CI
  * Fix t0170-dht sharness test. (@chriscool)
  * Increase timeout in t0060-daemon sharness test. (@Kubuxu)
  * Have CircleCI use `make deps` instead of `gx` directly. (@whyrusleeping)


### 0.4.1 - 2016-04-25

This is a patch release that fixes a few bugs, and adds a few small (but not
insignificant) features. The primary reason for this release is the listener
hang bugfix that was shipped in the 0.4.0 release.

* Features
  * implementated ipfs object diff (@whyrusleeping)
  * allow promises (used in get, refs) to fail (@whyrusleeping)

* Tool changes
  * Adds 'toolkit_upgrade' to the makefile help target (@achin)

* General Codebase
  * Use extracted go-libp2p-crypto, -secio, -peer packages (@lgierth)
  * Update go-libp2p (@lgierth)
  * Fix package manifest fields (@lgierth)
  * remove incfusever dead-code (@whyrusleeping)
  * remove a ton of unused godeps (@whyrusleeping)
  * metrics: add prometheus back (@lgierth)
  * clean up dead code and config fields (@whyrusleeping)
  * Add log events when blocks are added/removed from the blockstore (@michealmure)
  * repo: don't create logs directory, not used any longer (@lgierth)

* Bugfixes
  * fixed ipfs name resolve --local multihash error (@pfista)
  * ipfs patch commands won't return null links field anymore (@whyrusleeping)
  * Make non recursive resolve print the result (@Kubuxu)
  * Output dirs on ipfs add -rn (@noffle)
  * update libp2p dep to fix hanging listeners problem (@whyrusleeping)
  * Fix Swarm.AddrFilters config setting with regard to `/ip6` addresses (@lgierth)
  * fix dht command key escaping (@whyrusleeping)

* Testing
  * Adds tests to make sure 'object patch' writes. (@noffle)
  * small sharness test for promise failure checking (@whyrusleeping)
  * sharness/Makefile: clean all BINS when cleaning (@chriscool)

* Documentation
  * Fix disconnect argument description (@richardlitt)
  * Added a note about swarm disconnect (@richardlitt)
  * Also fixed syntax for comment (@richardlitt)
  * Alphabetized swarm subcmds (@richardlitt)
  * Added note to ipfs stats bw interval option (@richardlitt)
  * Small syntax changes to repo stat man (@richardlitt)
  * update log command help text (@pfista)
  * Added a long description to add (@richardlitt)
  * Edited object patch set-data doc (@richardlitt)
  * add roadmap.md (@Jeromy)
  * Adds files api cmd to helptext (@noffle)


### 0.4.0 - 2016-04-05

This is a major release with plenty of new features and bugfixes.
It also includes breaking changes which make it incompatible with v0.3.x
on the networking layer.

* Major Changes
  * Multistream
    * The addition of multistream is a breaking change on the networking layer,
      but gives IPFS implementations the ability to mix and match different
      stream multiplexers, e.g. yamux, spdystream, or muxado.
      This adds a ton of flexibility on one of the lower layers of the protocol,
      and will help us avoid further breaking protocol changes in the future.
  * Files API
    * The new `files` command and API allow a program to interact with IPFS
      using familiar filesystem operations, namely: creating directories,
      reading, writing, and deleting files, listing out different directories,
      and so on. This feature enables any other application that uses a
      filesystem-like backend for storage, to use IPFS as its storage driver
      without having change the application logic at all.
  * Gx
    * go-ipfs now uses [gx](https://github.com/whyrusleeping/gx) to manage its
      dependencies. This means that under the hood, go-ipfs's dependencies are
      backed by IPFS itself! It also means that go-ipfs is no longer installed
      using `go get`. Use `make install` instead.
* New Features
  * Web UI
    * Update to new version which is compatible with 0.4.0. (@dignifiedquire)
  * Networking
    * Implement uTP transport. (@whyrusleeping)
    * Allow multiple addresses per configured bootstrap node. (@whyrusleeping)
  * IPNS
    * Improve IPNS resolution performance. (@whyrusleeping)
    * Have dnslink prefer `TXT _dnslink.example.com`, allows usage of CNAME records. (@Kubuxu)
    * Prevent `ipfs name publish` when `/ipns` is mounted. (@noffle)
  * Repo
    * Improve performance of `ipfs add`. (@whyrusleeping)
    * Add `Datastore.NoSync` config option for flatfs. (@rht)
    * Implement mark-and-sweep GC. (@whyrusleeping)
    * Allow for GC during `ipfs add`. (@whyrusleeping)
    * Add `ipfs repo stat` command. (@tmg, @diasdavid)
  * General
    * Add support for HTTP OPTIONS requests. (@lidel)
    * Add `ipfs diag cmds` to view active API requests (@whyrusleeping)
    * Add an `IPFS_LOW_MEM` environment veriable which relaxes Bitswap's memory usage. (@whyrusleeping)
    * The Docker image now lives at `ipfs/go-ipfs` and has been completely reworked. (@lgierth)
* Security fixes
  * The gateway path prefix added in v0.3.10 was vulnerable to cross-site
    scripting attacks. This release introduces a configurable list of allowed
    path prefixes. It's called `Gateway.PathPrefixes` and takes a list of
    strings, e.g. `["/blog", "/foo/bar"]`. The v0.3.x line will not receive any
    further updates, so please update to v0.4.0 as soon as possible. (@lgierth)
* Incompatible Changes
  * Install using `make install` instead of `go get` (@whyrusleeping)
  * Rewrite pinning to store pins in IPFS objects. (@tv42)
  * Bump fs-repo version to 3. (@whyrusleeping)
  * Use multistream muxer (@whyrusleeping)
  * The default for `--type` in `ipfs pin ls` is now `all`. (@chriscool)
* Bug Fixes
  * Remove msgio double wrap. (@jbenet)
  * Buffer msgio. (@whyrusleeping)
  * Perform various fixes to the FUSE code. (@tv42)
  * Compute `ipfs add` size in background to not stall add operation. (@whyrusleeping)
  * Add option to have `ipfs add` include top-level hidden files. (@noffle)
  * Fix CORS checks on the API. (@rht)
  * Fix `ipfs update` error message. (@tomgg)
  * Resolve paths in `ipfs pin rm` without network lookup. (@noffle)
  * Detect FUSE unmounts and track mount state. (@noffle)
  * Fix go1.6rc2 panic caused by CloseNotify being called from wrong goroutine. (@rwcarlsen)
  * Bump DHT kvalue from 10 to 20. (@whyrusleeping)
  * Put public key and IPNS entry to DHT in parallel. (@whyrusleeping)
  * Fix panic in CLI argument parsing. (@whyrusleeping)
  * Fix range error by using larger-than-zero-length buffer. (@noffle)
  * Fix yamux hanging issue by increasing AcceptBacklog. (@whyrusleeping)
  * Fix double Transport-Encoding header bug. (@whyrusleeping)
  * Fix uTP panic and file descriptor leak. (@whyrusleeping)
* Tool Changes
  * Add `--pin` option to `ipfs add`, which defaults to `true` and allows `--pin=false`. (@eminence)
  * Add arguments to `ipfs pin ls`. (@chriscool)
  * Add `dns` and `resolve` commands to read-only API. (@Kubuxu)
  * Add option to display headers for `ipfs object links`. (@palkeo)
* General Codebase Changes
  * Check Golang version in Makefile. (@chriscool)
  * Improve Makefile. (@tomgg)
  * Remove dead Jenkins CI code. (@lgierth)
  * Add locking interface to blockstore. (@whyrusleeping)
  * Add Merkledag FetchGraph and EnumerateChildren. (@whyrusleeping)
  * Rename Lock/RLock to GCLock/PinLock. (@jbenet)
  * Implement pluggable datastore types. (@tv42)
  * Record datastore metrics for non-default datastores. (@tv42)
  * Allow multistream to have zero-rtt stream opening. (@whyrusleeping)
  * Refactor `ipnsfs` into a more generic and well tested `mfs`. (@whyrusleeping)
  * Grab more peers if bucket doesn't contain enough. (@whyrusleeping)
  * Use CloseNotify in gateway. (@whyrusleeping)
  * Flatten multipart file transfers. (@whyrusleeping)
  * Send updated DHT record fixes to peers who sent outdated records. (@whyrusleeping)
  * Replace go-psutil with go-sysinfo. (@whyrusleeping)
  * Use ServeContent for index.html. (@AtnNn)
  * Refactor `object patch` API to not store data in URL. (@whyrusleeping)
  * Use mfs for `ipfs add`. (@whyrusleeping)
  * Add `Server` header to API responses. (@Kubuxu)
  * Wire context directly into HTTP requests. (@rht)
  * Wire context directly into GetDAG operations within GC. (@rht)
  * Vendor libp2p using gx. (@whyrusleeping)
  * Use gx vendored packages instead of Godeps. (@whyrusleeping)
  * Simplify merkledag package interface to ease IPLD inclusion. (@mildred)
  * Add default option value support to commands lib. (@whyrusleeping)
  * Refactor merkledag fetching methods. (@whyrusleeping)
  * Use net/url to escape paths within Web UI. (@noffle)
  * Deprecated key.Pretty(). (@MichealMure)
* Documentation
  * Fix and update help text for **every** `ipfs` command. (@RichardLitt)
  * Change sample API origin settings from wildcard (`*`) to `example.com`. (@Kubuxu)
  * Improve documentation of installation process in README. (@whyrusleeping)
  * Improve windows.md. (@chriscool)
  * Clarify instructions for installing from source. (@noffle)
  * Make version checking more robust. (@jedahan)
  * Assert the source code is located within GOPATH. (@whyrusleeping)
  * Remove mentions of `/dns` from `ipfs dns` command docs. (@lgierth)
* Testing
  * Refactor iptb tests. (@chriscool)
  * Improve t0240 sharness test. (@chriscool)
  * Make bitswap tests less flaky. (@whyrusleeping)
  * Use TCP port zero for ipfs daemon in sharness tests. (@whyrusleeping)
  * Improve sharness tests on AppVeyor. (@chriscool)
  * Add a pause to fix timing on t0065. (@whyrusleeping)
  * Add support for arbitrary TCP ports to t0060-daemon.sh. (@noffle)
  * Make t0060 sharness test use TCP port zero. (@whyrusleeping)
  * Randomized ipfs stress testing via randor (@dignifiedquire)
  * Stress test pinning and migrations (@whyrusleeping)

### 0.3.11 - 2016-01-12

This is the final ipfs version before the transition to v0.4.0.
It introduces a few stability improvements, bugfixes, and increased
test coverage.

* Features
  * Add 'get' and 'patch' to the allowed gateway commands (@whyrusleeping)
  * Updated webui version (@dignifiedquire)

* BugFixes
  * Fix path parsing for add command (@djdv)
  * namesys: Make paths with multiple segments work. Fixes #2059 (@Kubuxu)
  * Fix up panic catching in http handler funcs (@whyrusleeping)
  * Add correct access control headers to the default api config (@dignifiedquire)
  * Fix closenotify by not sending empty file set (@whyrusleeping)

* Tool Changes
  * Have install.sh use the full path to ipfs binary if detected (@jedahan)
  * Install daemon system-wide if on El Capitan (@jedahan)
  * makefile: add -ldflags to install and nofuse tasks (@lgierth)

* General Codebase
  * Clean up http client code (@whyrusleeping)
  * Move api version check to header (@rht)

* Documentation
  * Improved release checklist (@jbenet)
  * Added quotes around command in long description (@RichardLitt)
  * Added a shutdown note to daemon description (@RichardLitt)

* Testing
  * t0080: improve last tests (@chriscool)
  * t0080: improve 'ipfs refs --unique' test (@chriscool)
  * Fix t.Fatal usage in goroutines (@chriscool)
  * Add docker testing support to sharness (@chriscool)
  * sharness: add t0300-docker-image.sh (@chriscool)
  * Included more namesys tests. (@Kubuxu)
  * Add sharness test to verify requests look good (@whyrusleeping)
  * Re-enable ipns sharness test now that iptb is fixed (@whyrusleeping)
  * Force use of ipv4 in test (@whyrusleeping)
  * Travis-CI: use go 1.5.2 (@jbenet)

### 0.3.10 - 2015-12-07

This patch update introduces the 'ipfs update' command which will be used for
future ipfs updates along with a few other bugfixes and documentation
improvements.


* Features
  * support for 'ipfs update' to call external binary (@whyrusleeping)
  * cache ipns entries to speed things up a little (@whyrusleeping)
  * add option to version command to print repo version (@whyrusleeping)
  * Add in some more notifications to help profile queries (@whyrusleeping)
  * gateway: add path prefix for directory listings (@lgierth)
  * gateway: add CurrentCommit to /version (@lgierth)

* BugFixes
  * set data and links nil if not present (@whyrusleeping)
  * fix log hanging issue, and implement close-notify for commands (@whyrusleeping)
  * fix dial backoff (@whyrusleeping)
  * proper ndjson implementation (@whyrusleeping)
  * seccat: fix secio context (@lgierth)
  * Add newline to end of the output for a few commands. (@nham)
  * Add fixed period repo GC + test (@rht)

* Tool Changes
  * Allow `ipfs cat` on ipns path (@rht)

* General Codebase
  * rewrite of backoff mechanism (@whyrusleeping)
  * refactor net code to use transports, in rough accordance with libp2p (@whyrusleeping)
  * disable building fuse stuff on windows (@whyrusleeping)
  * repo: remove Log config (@lgierth)
  * commands: fix description of --api (@lgierth)

* Documentation
  * --help: Add a note on using IPFS_PATH to the footer of the helptext.  (@sahib)
  * Moved email juan to ipfs/contribute (@richardlitt)
  * Added commit sign off section (@richardlitt)
  * Added a security section (@richardlitt)
  * Moved TODO doc to issue #1929 (@richardlitt)

* Testing
  * gateway: add tests for /version (@lgierth)
  * Add gc auto test (@rht)
  * t0020: cleanup dir with bad perms (@chriscool)

Note: this commit introduces fixed-period repo gc, which will trigger gc
after a fixed period of time. This feature is introduced now, disabled by
default, and can be enabled with `ipfs daemon --enable-gc`. If all goes well,
in the future, it will be enabled by default.

### 0.3.9 - 2015-10-30

This patch update includes a good number of bugfixes, notably, it fixes
builds on windows, and puts newlines between streaming json objects for a
proper nsjon format.

* Features
  * Writable gateway enabled again (@cryptix)

* Bugfixes
  * fix windows builds (@whyrusleeping)
  * content type on command responses default to text (@whyrusleeping)
  * add check to makefile to ensure windows builds dont fail silently (@whyrusleeping)
  * put newlines between streaming json output objects (@whyrusleeping)
  * fix streaming output to flush per write (@whyrusleeping)
  * purposely fail builds pre go1.5 (@whyrusleeping)
  * fix ipfs id <self> (@whyrusleeping)
  * fix a few race conditions in mocknet (@whyrusleeping)
  * fix makefile failing when not in a git repo (@whyrusleeping)
  * fix cli flag orders (long, short) (@rht)
  * fix races in http cors (@miolini)
  * small webui update (some bugfixes) (@jbenet)

* Tool Changes
  * make swarm connect return an error when it fails (@whyrusleeping)
  * Add short flag for `ipfs ls --headers` (v for verbose) (@rht)

* General Codebase
  * bitswap: clean log printf and humanize dup data count (@cryptix)
  * config: update pluto's peerID (@lgierth)
  * config: update bootstrap list hostname (@lgierth)

* Documentation
  * Pared down contribute to link to new go guidelines (@richardlitt)

* Testing
  * t0010: add tests for 'ipfs commands --flags' (@chriscool)
  * ipns_test: fix namesys.NewNameSystem() call (@chriscool)
  * t0060: fail if no nc (@chriscool)

### 0.3.8 - 2015-10-09

This patch update includes changes to make ipns more consistent and reliable,
symlink support in unixfs, mild performance improvements, new tooling features,
a plethora of bugfixes, and greatly improved tests.

NOTICE: Version 0.3.8 also requires golang version 1.5.1 or higher.

* Bugfixes
  * refactor ipns to be more consistent and reliable (@whyrusleeping)
  * fix 'ipfs refs' json output (@whyrusleeping)
  * fix setting null config maps (@rht)
  * fix output of dht commands (@whyrusleeping)
  * fix NAT spam dialing (@whyrusleeping)
  * fix random panics on 32 bit systems (@whyrusleeping)
  * limit total number of network fd's (@whyrusleeping)
  * fix http api content type (@WeMeetAgain)
  * fix writing of api file for port zero daemons (@whyrusleeping)
  * windows connection refused fixes (@mjanczyk)
  * use go1.5's built in trailers, no more failures (@whyrusleeping)
  * fix random bitswap hangs (@whyrusleeping)
  * rate limit fd usage (@whyrusleeping)
  * fix panic in bitswap ratelimiting (@whyrusleeping)

* Tool Changes
  * --empty-repo option for init (@prusnak)
  * implement symlinks (@whyrusleeping)
  * improve cmds lib files processing (@rht)
  * properly return errors through commands (@whyrusleeping)
  * bitswap unwant command (@whyrusleeping)
  * tar add/cat commands (@whyrusleeping)
  * fix gzip compression in get (@klauspost)
  * bitswap stat logs wasted bytes (@whyrusleeping)
  * resolve command now uses core.Resolve (@rht)
  * add `--local` flag to 'name resolve' (@whyrusleeping)
  * add `ipfs diag sys` command for debugging help (@whyrusleeping)

* General Codebase
  * improvements to dag editor (@whyrusleeping)
  * swarm IPv6 in default config (Baptiste Jonglez)
  * improve dir listing css (@rht)
  * removed elliptic.P224 usage (@prusnak)
  * improve bitswap providing speed (@jbenet)
  * print panics that occur in cmds lib (@whyrusleeping)
  * ipfs api check test fixes (@rht)
  * update peerstream and datastore (@whyrusleeping)
  * cleaned up tar-reader code (@jbenet)
  * write context into coreunix.Cat (@rht)
  * move assets to separate repo (@rht)
  * fix proc/ctx wiring in bitswap (@jbenet)
  * rabin fingerprinting chunker (@whyrusleeping)
  * better notification on daemon ready (@rht)
  * coreunix cat cleanup (@rht)
  * extract logging into go-log (@whyrusleeping)
  * blockservice.New no longer errors (@whyrusleeping)
  * refactor ipfs get (@rht)
  * readonly api on gateway (@rht)
  * cleanup context usage all over (@rht)
  * add xml decoding to 'object put' (@ForrestWeston)
  * replace nodebuilder with NewNode method (@whyrusleeping)
  * add metrics to http handlers (@lgierth)
  * rm blockservice workers (@whyrusleeping)
  * decompose maybeGzWriter (@rht)
  * makefile sets git commit sha on build (@CaioAlonso)

* Documentation
  * add contribute file (@RichardLitt)
  * add go devel guide to contribute.md (@whyrusleeping)

* Testing
  * fix mock notifs test (@whyrusleeping)
  * test utf8 with object cmd (@chriscool)
  * make mocknet conn close idempotent (@jbenet)
  * fix fuse tests (@pnelson)
  * improve sharness test quoting (@chriscool)
  * sharness tests for chunker and add-cat (@rht)
  * generalize peerid check in sharness (@chriscool)
  * test_cmp argument cleanup (@chriscool)

### 0.3.7 - 2015-08-02

This patch update fixes a problem we introduced in 0.3.6 and did not
catch: the webui failed to work with out-of-the-box CORS configs.
This has been fixed and now should work correctly. @jbenet

### 0.3.6 - 2015-07-30

This patch improves the resource consumption of go-ipfs,
introduces a few new options on the CLI, and also
fixes (yet again) windows builds.

* Resource consumption:
  * fixed goprocess memory leak @rht
  * implement batching on datastore @whyrusleeping
  * Fix bitswap memory leak @whyrusleeping
  * let bitswap ignore temporary write errors @whyrusleeping
  * remove logging to disk in favor of api endpoint @whyrusleeping
  * --only-hash option for add to skip writing to disk @whyrusleeping

* Tool changes
  * improved `ipfs daemon` output with all addresses @jbenet
  * improved `ipfs id -f` output, added `<addrs>` and  `\n \t` support @jbenet
  * `ipfs swarm addrs local` now shows the local node's addrs @jbenet
  * improved config json parsing @rht
  * improved Dockerfile to use alpine linux @Luzifer @lgierth
  * improved bash completion @MichaelMure
  * Improved 404 for gateway @cryptix
  * add unixfs ls to list correct filesizes @wking
  * ignore hidden files by default @gatesvp
  * global --timeout flag @whyrusleeping
  * fix random API failures by closing resp bodies @whyrusleeping
  * ipfs swarm filters @whyrusleeping
  * api returns errors in http trailers @whyrusleeping @jbenet
  * `ipfs patch` learned to create intermediate nodes @whyrusleeping
  * `ipfs object stat` now shows Hash @whyrusleeping
  * `ipfs cat` now clears progressbar on exit @rht
  * `ipfs add -w -r <dir>` now wraps directories @jbenet
  * `ipfs add -w <file1> <file2>` now wraps with one dir @jbenet
  * API + Gateway now support arbitrary HTTP Headers from config @jbenet
  * API now supports CORS properly from config @jbenet
  * **Deprecated:** `API_ORIGIN` env var (use config, see `ipfs daemon --help`) @jbenet

* General Codebase
  * `nofuse` tag for windows @Luzifer
  * improved `ipfs add` code @gatesvp
  * started requiring license trailers @chriscool @jbenet
  * removed CtxCloser for goprocess @rht
  * remove deadcode @lgierth @whyrusleeping
  * reduced number of logging libs to 2 (soon to be 1) @rht
  * dial address filtering @whyrusleeping
  * prometheus metrics @lgierth
  * new index page for gateway @krl @cryptix
  * move ping to separate protocol @whyrusleeping
  * add events to bitswap for a dashboard @whyrusleeping
  * add latency and bandwidth options to mocknet @heems
  * levenshtein distance cmd autosuggest @sbruce
  * refactor/cleanup of cmds http handler @whyrusleeping
  * cmds http stream reports errors in trailers @whyrusleeping

* Bugfixes
  * fixed path resolution and validation @rht
  * fixed `ipfs get -C` output and progress bar @rht
  * Fixed install pkg dist bug @jbenet @Luzifer
  * Fix `ipfs get` silent failure   @whyrusleeping
  * `ipfs get` tarx no longer times out @jbenet
  * `ipfs refs -r -u` is now correct @gatesvp
  * Fix `ipfs add -w -r <dir>` wrapping bugs @jbenet
  * Fixed FUSE unmount failures @jbenet
  * Fixed `ipfs log tail` command (api + cli) @whyrusleeping

* Testing
  * sharness updates @chriscool
  * ability to disable secio for testing @jbenet
  * fixed many random test failures, more reliable CI @whyrusleeping
  * Fixed racey notifier failures @whyrusleeping
  * `ipfs refs -r -u` test cases @jbenet
  * Fix failing pinning test @jbenet
  * Better CORS + Referer tests @jbenet
  * Added reversible gc test @rht
  * Fixed bugs in FUSE IPNS tests @whyrusleeping
  * Fixed bugs in FUSE IPFS tests @jbenet
  * Added `random-files` tool for easier sharness tests @jbenet

* Documentation
  * Add link to init system examples @slang800
  * Add CORS documentation to daemon init @carver  (Note: this will change soon)

### 0.3.5 - 2015-06-11

This patch improves overall stability and performance

* added 'object patch' and 'object new' commands @whyrusleeping
* improved symmetric NAT avoidance @jbenet
* move util.Key to blocks.Key @whyrusleeping
* fix memory leak in provider store @whyrusleeping
* updated webui to 0.2.0 @krl
* improved bitswap performance @whyrusleeping
* update fuse lib @cryptix
* fix path resolution @wking
* implement test_seq() in sharness @chriscool
* improve parsing of stdin for commands @chriscool
* fix 'ipfs refs' failing silently @whyrusleeping
* fix serial dialing bug @jbenet
* improved testing @chriscool @rht @jbenet
* fixed domain resolving @luzifer
* fix parsing of unwanted stdin @lgierth
* added CORS handlers to gateway @NodeGuy
* added `ipfs daemon --unrestricted-api` option @krl
* general cleanup of dependencies

### 0.3.4 - 2015-05-10

* fix ipns append bug @whyrusleeping
* fix out of memory panic @whyrusleeping
* add in expvar metrics @tv42
* bitswap improvements @whyrusleeping
* fix write-cache in blockstore @tv42
* vendoring cleanup @cryptix
* added `launchctl` plist for OSX @grncdr
* improved Dockerfile, changed root and mount paths @ehd
* improved `pin ls` output to show types @vitorbaptista

### 0.3.3 - 2015-04-28

This patch update fixes various issues, in particular:
- windows support (0.3.0 had broken it)
- commandline parses spaces correctly.

* much improved commandline parsing by @AtnNn
* improved dockerfile by @luzifer
* add cmd cleanup by @wking
* fix flatfs windows support by @tv42 and @gatesvp
* test case improvements by @chriscool
* ipns resolution timeout bug fix by @whyrusleeping
* new cluster tests with iptb by @whyrusleeping
* fix log callstack printing bug by @whyrusleeping
* document bash completiong by @dylanPowers

### 0.3.2 - 2015-04-22

This patch update implements multicast dns as well as fxing a few test issues.

* implment mdns peer discovery @whyrusleeping
* fix mounting issues in sharness tests @chriscool

### 0.3.1 - 2015-04-21

This patch update fixes a few bugs:

* harden shutdown logic by @torarnv
* daemon locking fixes by @travisperson
* don't re-add entire dirs by @whyrusleeping
* tests now wait for graceful shutdown by @jbenet
* default key size is now 2048 by @jbenet

### 0.3.0 - 2015-04-20

We've just released version 0.3.0, which contains many
performance improvements, bugfixes, and new features.
Perhaps the most noticeable change is moving block storage
from leveldb to flat files in the filesystem.

What to expect:

* _much faster_ performance

* Repo format 2
  * moved default location from ~/.go-ipfs -> ~/.ipfs
  * renamed lock filename daemon.lock -> repo.lock
  * now using a flat-file datastore for local blocks

* Fixed lots of bugs
  * proper ipfs-path in various commands
  * fixed two pinning bugs (recursive pins)
  * increased yamux streams window (for speed)
  * increased bitswap workers (+ env var)
  * fixed memory leaks
  * ipfs add error returns
  * daemon exit bugfix
  * set proper UID and GID on fuse mounts

* Gateway
  * Added support for HEAD requests

* configuration
  * env var to turn off SO_REUSEPORT: IPFS_REUSEPORT=false
  * env var to increase bitswap workers: IPFS_BITSWAP_TASK_WORKERS=n

* other
  * bash completion is now available
  * ipfs stats bw -- bandwidth meetering

And many more things.

### 0.2.3 - 2015-03-01

* Alpha Release

### 2015-01-31:

* bootstrap addresses now have .../ipfs/... in format
  config file Bootstrap field changed accordingly. users
  can upgrade cleanly with:

      ipfs bootstrap >boostrap_peers
      ipfs bootstrap rm --all
      <install new ipfs>
      <manually add .../ipfs/... to addrs in bootstrap_peers>
      ipfs bootstrap add <bootstrap_peers
