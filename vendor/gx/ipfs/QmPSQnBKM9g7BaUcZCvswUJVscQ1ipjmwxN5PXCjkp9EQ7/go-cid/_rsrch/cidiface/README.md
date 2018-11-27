What golang Kinds work best to implement CIDs?
==============================================

There are many possible ways to implement CIDs.  This package explores them.

### criteria

There's a couple different criteria to consider:

- We want the best performance when operating on the type (getters, mostly);
- We want to minimize the number of memory allocations we need;
- We want types which can be used as map keys, because this is common.

The priority of these criteria is open to argument, but it's probably
mapkeys > minalloc > anythingelse.
(Mapkeys and minalloc are also quite entangled, since if we don't pick a
representation that can work natively as a map key, we'll end up needing
a `KeyRepr()` method which gives us something that does work as a map key,
an that will almost certainly involve a malloc itself.)

### options

There are quite a few different ways to go:

- Option A: CIDs as a struct; multihash as bytes.
- Option B: CIDs as a string.
- Option C: CIDs as an interface with multiple implementors.
- Option D: CIDs as a struct; multihash also as a struct or string.
- Option E: CIDs as a struct; content as strings plus offsets.
- Option F: CIDs as a struct wrapping only a string.

The current approach on the master branch is Option A.

Option D is distinctive from Option A because multihash as bytes transitively
causes the CID struct to be non-comparible and thus not suitable for map keys
as per https://golang.org/ref/spec#KeyType .  (It's also a bit more work to
pursue Option D because it's just a bigger splash radius of change; but also,
something we might also want to do soon, because we *do* also have these same
map-key-usability concerns with multihash alone.)

Option E is distinctive from Option D because Option E would always maintain
the binary format of the cid internally, and so could yield it again without
malloc, while still potentially having faster access to components than
Option B since it wouldn't need to re-parse varints to access later fields.

Option F is actually a varation of Option B; it's distinctive from the other
struct options because it is proposing *literally* `struct{ x string }` as
the type, with no additional fields for components nor offsets.

Option C is the avoid-choices choice, but note that interfaces are not free;
since "minimize mallocs" is one of our major goals, we cannot use interfaces
whimsically.

Note there is no proposal for migrating to `type Cid []bytes`, because that
is generally considered to be strictly inferior to `type Cid string`.


Discoveries
-----------

### using interfaces as map keys forgoes a lot of safety checks

Using interfaces as map keys pushes a bunch of type checking to runtime.
E.g., it's totally valid at compile time to push a type which is non-comparable
into a map key; it will panic at *runtime* instead of failing at compile-time.

There's also no way to define equality checks between implementors of the
interface: golang will always use its innate concept of comparison for the
concrete types.  This means its effectively *never safe* to use two different
concrete implementations of an interface in the same map; you may add elements
which are semantically "equal" in your mind, and end up very confused later
when both impls of the same "equal" object have been stored.

### sentinel values are possible in any impl, but some are clearer than others

When using `*Cid`, the nil value is a clear sentinel for 'invalid';
when using `type Cid string`, the zero value is a clear sentinel;
when using `type Cid struct` per Option A or D... the only valid check is
for a nil multihash field, since version=0 and codec=0 are both valid values.
When using `type Cid struct{string}` per Option F, zero is a clear sentinel.

### usability as a map key is important

We already covered this in the criteria section, but for clarity:

- Option A: ❌
- Option B: ✔
- Option C: ~ (caveats, and depends on concrete impl)
- Option D: ✔
- Option E: ✔
- Option F: ✔

### living without offsets requires parsing

Since CID (and multihash!) are defined using varints, they require parsing;
we can't just jump into the string at a known offset in order to yield e.g.
the multicodec number.

In order to get to the 'meat' of the CID (the multihash content), we first
must parse:

- the CID version varint;
- the multicodec varint;
- the multihash type enum varint;
- and the multihash length varint.

Since there are many applications where we want to jump straight to the
multihash content (for example, when doing CAS sharding -- see the
[disclaimer](https://github.com/multiformats/multihash#disclaimers) about
bias in leading bytes), this overhead may be interesting.

How much this overhead is significant is hard to say from microbenchmarking;
it depends largely on usage patterns. If these traversals are a significant
timesink, it would be an argument for Option D/E.
If these traversals are *not* a significant timesink, we might be wiser
to keep to Option B/F, because keeping a struct full of offsets will add several
words of memory usage per CID, and we keep a *lot* of CIDs.

### interfaces cause boxing which is a significant performance cost

See `BenchmarkCidMap_CidStr` and friends.

Long story short: using interfaces *anywhere* will cause the compiler to
implicitly generate boxing and unboxing code (e.g. `runtime.convT2E`);
this is both another function call, and more concerningly, results in
large numbers of unbatchable memory allocations.

Numbers without context are dangerous, but if you need one: 33%.
It's a big deal.

This means attempts to "use interfaces, but switch to concrete impls when
performance is important" are a red herring: it doesn't work that way.

This is not a general inditement against using interfaces -- but
if a situation is at the scale where it's become important to mind whether
or not pointers are a performance impact, then that situation also
is one where you have to think twice before using interfaces.

### struct wrappers can be used in place of typedefs with zero overhead

See `TestSizeOf`.

Using the `unsafe.Sizeof` feature to inspect what the Go runtime thinks,
we can see that `type Foo string` and `type Foo struct{x string}` consume
precisely the same amount of memory.

This is interesting because it means we can choose between either
type definition with no significant overhead anywhere we use it:
thus, we can choose freely between Option B and Option F based on which
we feel is more pleasant to work with.

Option F (a struct wrapper) means we can prevent casting into our Cid type.
Option B (typedef string) can be declared a `const`.
Are there any other concerns that would separate the two choices?

### one way or another: let's get rid of that star

We should switch completely to handling `Cid` and remove `*Cid` completely.
Regardless of whether we do this by migrating to interface, or string
implementations, or simply structs with no pointers... once we get there,
refactoring to any of the *others* can become a no-op from the perspective
of any downstream code that uses CIDs.

(This means all access via functions, never references to fields -- even if
we were to use a struct implementation.  *Pretend* there's a interface,
in other words.)

There are probably `gofix` incantations which can help us with this migration.
