CHANGELOG
=========

Here is collected some brief notes on major changes over time, sorted by tag in which they are first available.

Of course for the "detailed changelog", you can always check the commit log!  But hopefully this summary _helps_.

Note about version numbering: All release tags are in the "v0.${x}" range.  _We do not expect to make a v1 release._
Nonetheless, this should not be taken as a statement that the library isn't _usable_ already.
Much of this code is used in other libraries and products, and we do take some care about making changes.
(If you're ever wondering about stability of a feature, ask -- or contribute more tests ;))

- [Planned/Upcoming Changes](#planned-upcoming-changes)
- [Changes on master branch but not yet Released](#unreleased-on-master)
- [Released Changes Log](#released-changes)


Planned/Upcoming Changes
------------------------

Here are some outlines of changes we intend to make that affect the public API:

- Something about linking...
	- But it's not clear what.  See https://github.com/ipld/go-ipld-prime/issues/55 for discussion.
	- If nothing else, `ipld.Loader` and `ipld.Storer` will probably change to get "Link" somewhere in their name, because that's turned out to be how we're always colloquially referring to them.

This is not an exhaustive list of planned changes, and does not include any internal changes, new features, performance improvements, and so forth.
It's purely a list of things you might want to know about as a downstream consumer planning your update cycles.

We will make these changes "soon" (for some definition of "soon").
They are currently not written on the master branch.
The definition of "soon" may vary, in service of a goal to sequence any public API changes in a way that's smooth to migrate over, and make those changes appear at an overall bearable chronological frequency.
Tagged releases will be made when any of these changes land, so you can upgrade intentionally.


Unreleased on master
--------------------

Changes here are on the master branch, but not in any tagged release yet.
When a release tag is made, this block of bullet points will just slide down to the [Released Changes](#released-changes) section.

- _nothing yet :)_


Released Changes
----------------

### v0.5.0

v0.5.0 is a small release -- it just contains a bunch of renames.
There are _no_ semantic changes bundled with this (it's _just_ renames) so this should be easy to absorb.

- Renamed: `NodeStyle` -> `NodePrototype`.
	- Reason: it seems to fit better!  See https://github.com/ipld/go-ipld-prime/issues/54 for a full discussion.
	- This should be a "sed refactor" -- the change is purely naming, not semantics, so it should be easy to update your code for.
	- This also affects some package-scoped vars named `Style`; they're accordingly also renamed to `Prototype`.
	- This also affects several methods such as `KeyStyle` and `ValueStyle`; they're accordingly also renamed to `KeyPrototype` and `ValuePrototype`.
- Renamed: `(Node).Lookup{Foo}` -> `(Node).LookupBy{Foo}`.
	- Reason: The former phrasing makes it sound like the "{Foo}" component of the name describes what it returns, but in fact what it describes is the type of the param (which is necessary, since Golang lacks function overloading parametric polymorphism).  Adding the preposition should make this less likely to mislead (even though it does make the method name moderately longer).
	- This should be a "sed refactor" -- the change is purely naming, not semantics, so it should be easy to update your code for.
- Renamed: `(Node).Lookup` -> `(Node).LookupNode`.
	- Reason: The shortest and least-qualified name, 'Lookup', should be reserved for the best-typed variant of the method, which is only present on codegenerated types (and not present on the Node interface at all, due to Golang's limited polymorphism).
	- This should be a "sed refactor" -- the change is purely naming, not semantics, so it should be easy to update your code for.  (The change itself in the library was fairly literally `s/Lookup(/LookupNode(/g`, and then `s/"Lookup"/"LookupNode"/g` to catch a few error message strings, so consumers shouldn't have it much harder.)
	- Note: combined with the above rename, this method overall becomes `(Node).LookupByNode`.
- Renamed: `ipld.Undef` -> `ipld.Absent`, and `(Node).IsUndefined` -> `(Node).IsAbsent`.
	- Reason: "absent" has emerged as a much, much better description of what this value means.  "Undefined" sounds nebulous and carries less meaning.  In long-form prose docs written recently, "absent" consistently fits the sentence flow much better.  Let's just adopt "absent" consistently and do away with "undefined".
	- This should be a "sed refactor" -- the change is purely naming, not semantics, so it should be easy to update your code for.


### v0.4.0

v0.4.0 contains some misceleanous features and documentation improvements -- perhaps most notably, codegen is re-introduced and more featureful than previous rounds -- but otherwise isn't too shocking.
This tag mostly exists as a nice stopping point before the next version coming up (which is planned to include several API changes).

- Docs: several new example functions should now appear in the godoc for how to use the linking APIs.
- Feature: codegen is back!  Use it if you dare.
	- Generated code is now up to date with the present versions of the core interfaces (e.g., its updated for the NodeAssembler world).
	- We've got a nice big feature table in the codegen package readme now!  Consult that to see which features of IPLD Schemas now have codegen support.
	- There are now several implemented and working (and robustly tested) examples of codegen for various representation strategies for the same types.  (For example, struct-with-stringjoin-representation.)  Neat!
	- This edition of codegen uses some neat tricks to not just maintain immutability contracts, but even prevent the creation of zero-value objects which could potentially be used to evade validation phases on objects that have validation rules.  (This is a bit experimental; we'll see how it goes.)
	- There are oodles and oodles of deep documentation of architecture design choices recorded in "HACKME_*" documents in the codegen package that you may enjoy if you want to contribute or understand why generated things are the way they are.
	- Testing infrastructure for codegen is now solid.  Running tests for the codegen package will: exercise the generation itself; AND make sure the generated code compiles; AND run behavioral tests against it: the whole gamut, all from regular `go test`.
	- The "node/gendemo" package contains a real example of codegen output... and it's connected to the same tests and benchmarks as other node implementations.  (Are the gen'd types fast?  yes.  yes they are.)
	- There's still lots more to go: interacting with the codegen system still requires writing code to interact with as a library, as we aren't shipping a CLI frontend to it yet; and many other features are still in development as well.  But you're welcome to take it for a spin if you're eager!
- Feature: introduce JSON Tables Codec ("JST"), in the `codec/jst` package.  This is a codec that emits bog-standard JSON, but leaning in on the non-semantic whitespace to produce aligned output, table-like, for pleasant human reading.  (If you've used `column -t` before in the shell: it's like that.)
	- This package may be a temporary guest in this repo; it will probably migrate to its own repo soon.  (It's a nice exercise of our core interfaces, though, so it incubated here.)
- I'm quietly shifting the versioning up to the 0.x range.  (Honestly, I thought it was already there, heh.)  That makes this this "v0.4".


### v0.0.3

v0.0.3 contained a massive rewrite which pivoted us to using NodeAssembler patterns.
Code predating this version will need significant updates to match; but, the performance improvements that result should be more than worth it.

- Constructing new nodes has a major pivot towards using "NodeAssembler" pattern: https://github.com/ipld/go-ipld-prime/pull/49
	- This was a massively breaking change: it pivoted from bottom-up composition to top-down assembly: allocating large chunks of structured memory up front and filling them in, rather than stitching together trees over fragmented heap memory with lots of pointers
- "NodeStyle" and "NodeBuilder" and "NodeAssembler" are all now separate concepts:
	- NodeStyle is more or less a builder factory (forgive me -- but it's important: you can handle these without causing allocations, and that matters).
	  Use NodeStyle to tell library functions what kind of in-memory representation you want to use for your data.  (Typically `basicnode.Style.Any` will do -- but you have the control to choose others.)
	- NodeBuilder allocates and begins the assembly of a value (or a whole tree of values, which may be allocated all at once).
	- NodeAssembler is the recursive part of assembling a value (NodeBuilder implements NodeAssembler, but everywhere other than the root, you only use the NodeAssembler interface).
- Assembly of trees of values now simply involves asking the assembler for a recursive node to give you assemblers for the keys and/or values, and then simply... using them.
	- This is much simpler (and also faster) to use than the previous system, which involved an awkward dance to ask about what kind the child nodes were, get builders for them, use those builders, then put the result pack in the parent, and so forth.
- Creating new maps and lists now accepts a size hint argument.
	- This isn't strictly enforced (you can provide zero, or even a negative number to indicate "I don't know", and still add data to the assembler), but may improve efficiency by reducing reallocation costs to grow structures if the size can be estimated in advance.
- Expect **sizable** performance improvements in this version, due to these interface changes.
- Some packages were renamed in an effort to improve naming consistency and feel:
	- The default node implementations have moved: expect to replace `impl/free` in your package imports with `node/basic` (which is an all around better name, anyway).
	- The codecs packages have moved: replace `encoding` with `codec` in your package imports (that's all there is to it; nothing else changed).
- Previous demos of code generation are currently broken / disabled / removed in this tag.
	- ...but they'll return in future versions, and you can follow along in branches if you wish.
- Bugfix: dag-cbor codec now correctly handles marshalling when bytes come after a link in the same object. [[53](https://github.com/ipld/go-ipld-prime/pull/53)]

### v0.0.2

- Many various performance improvements, fixes, and docs improvements.
- Many benchmarks and additional tests introduced.
- Includes early demos of parts of the schema system, and early demos of code generation.
- Mostly a checkpoint before beginning v0.0.3, which involved many large API reshapings.

### v0.0.1

- Our very first tag!
- The central `Node` and `NodeBuilder` interfaces are already established, as is `Link`, `Loader`, and so forth.
  You can already build generic data handling using IPLD Data Model concepts with these core interfaces.
- Selectors and traversals are available.
- Codecs for dag-cbor and dag-json are batteries-included in the repo.
- There was quite a lot of work done before we even started tagging releases :)
