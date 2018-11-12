code layout
===========

Package layout
--------------

- `refmt` -- main package.  All major interface types and helpful factory methods.
  - `json` -- `json.Serializer` and `json.Deserializer`
  - `cbor` -- `cbor.Serializer` and `cbor.Deserializer`
  - `obj` -- `obj.Marshaller` and `obj.Unmarshaller`
    - `atlas` -- types for describing how to `obj.*Marshaller`s should visit complex types.
  - `tok` -- token handling utils.  Many exported values, for use in sibling packages, but not often seen by users.

(Experienced go developers will probably already have noticed that putting core interfaces and factory methods in the same package is usually going to run aground on the no-cyclic-imports rule.
Fortunately, all of the concrete types the json/cbor/etc packages must import fit cleanly under the `tok` package.)

-----------
User-facing
-----------

- example code:
  ```
  refmt.NewJsonEncoder(stdout).Marshal(123)
  ```
  - Creates an `obj.Marshaller` as the `refmt.TokenSource` for walking the object (`123`).
  - Creates a `json.Marshaller` as the `refmt.TokenSink` outputting to `stdout`.
  - Both are placed into a `refmt.TokenPump`, which powers the rest of the transaction.
  - The `obj.Marshaller` sees the type of object it was given and immediately delegates to an `obj.marshalMachineLiteral`.
  - The `obj.marshalMachineLiteral` steps once, yields a token (it's going to be `TokenType` of `TInt`), and reports done.  There are no other stacked machines, so the `obj.Marshaller` overall reports done.
  - The `json.Encoder` is invoked and given a token by the `refmt.TokenPump`, and writes the number to `stdout`.  The `json.Encoder` knows that it just wrote a literal type, and since it's not deep in an object tree that's the end of a json entity, so it reports done.
  - The `refmt.TokenPump` sees both sides finished in unison, and returns done, with no error!

-----------------------
Token stream interfaces
-----------------------

Refmt handles every translation by converting things into a token stream,
then processing the token stream into the desired result format.

`refmt.TokenSource` and `refmt.TokenSink` describe how to produce and process token streams, respectively.
Listing their implementations effectively lists every format that refmt can convert to and from.

`Token`s the interal lingua franca of refmt.
A handful of special tokens signal the beginning and ending of maps and arrays.
All other values are their own tokens -- we simply use the address of the real data
(only primitives, of course; it's a stream, not a tree, after all).
Using the address of the real data avoids unnecessary memcopy operations, and
makes encoding to bytes a choice rather than a requirement.

- **TokenSource** *interface*

  Anything that emits a stream of tokens.

  Call a `TokenSource` repeatedly; each call will yield a token into the memory provided
  (and typically cause the concrete implementation to advance the `TokenSource`'s internal state by single step, as appropriate).
  Errors or end-of-stream are indicated by the return codes.

  - *Implementations*:
    - **json.Decoder** -- constructed with an `io.Reader`, from which (hopefully-)json-formatted bytes will be consumed and converted into tokens.
    - **cbor.Decoder** -- constructed with an `io.Reader`, from which (hopefully-)cbor-formatted bytes will be consumed and converted into tokens.
    - **obj.Marshaller** -- constructed with a reference to any object, which will be visited and all fields emitted one by one as tokens.

- **TokenSink** *interface*

  Anything that consumes a stream of tokens.

  Call a `TokenSink` repeatedly; each call will copy the token content into its new home
  (whatever that may be, whether a serial byte stream or a complex in-memory layout).
  Errors and expected end-of-stream are indicated by the return codes.

    - **json.Encoder** -- constructed with an `io.Writer`, to which json-formatted bytes are flushed as each token is received.
    - **cbor.Encoder** -- constructed with an `io.Writer`, to which cbor-formatted bytes are flushed as each token is received.
    - **obj.Unmarshaller** -- constructed with a reference to any object (or empty `interface{}`), which will be populated based on tokens received.

- **TokenPump** *struct*

  Joins a `TokenSource` and `TokenSink`, advancing them in lock-step, and looping until they're complete.

  The `TokenPump` will return one of:
    - an error, if any step function of either the source or sink returns an error;
    - an error if the sink expects to be done while the source is continuing;
    - or a done bool with nil error, if both the source and sink become done during the same step.

--------------------------
Object traversal internals
--------------------------

Object traversal is complex and requires its own substantial amount of machinery.

Users do not interact with any of these directly.

- **obj.Marshaller** *struct*

  Top-level control for a system that walks in-memory structures and emits tokens
  describing the object it's observing.

  View this like a program stack; `obj.MarshalMachine`s are the functions in the stack.

- **obj.MarshalMachine** *interface*

  A single state machine that emits tokens implements this.
  This interface makes the individual machines composable to `Marshaller`.

  In automata theory, `MarshalMachine` is generally a DFA FSM -- that is,
  it can operate with a finite amount of memory -- and we use the `Marshaller`
  to gather together `MarshalMachine`s into stacks, thus giving us a
  *pushdown automata*, which has sufficient power to handle recursive structure.

  In practical terms, restricting `MarshalMachine`s to a DFA means they
  can operate without allocating memory on the heap.  This confers a large
  bonus to overall performance.

  If a `Marshaller` is like a stack, and the `MarshalMachine` like a function,
  then each time you call the `MarshalMachine` is like stepping through a function one
  line (or one instruction) at a time.

  - *Implementations:*
    - **marshalMachineWildcard** -- turns any `interface{}` into tokens (works by looking up a more specific encode machine, then yielding to it).
    - **marshalMachineMapWildcard** -- turns a `map[K]V` into tokens (works for any type, using reflection).
    - **marshalMachineMapStringWildcard** -- turns a `map[string]interface{}` into tokens (faster than the reflection used in the wildcard system).
    - **marshalMachineLiteral** -- turns primitives like `int` and `string` into tokens (hardly even a DFA; only ever takes one step).
    - **marshalMachineStructAtlas** -- uses an `Atlas` to visit and emit tokens covering an arbitrary struct type.
    - **marshalMachinePolymorphicUnion** -- uses a `PolymorphAtlas` to look at an arbitrary value, emit a single-entry map, and trigger a more specific encode machine (the single key is presumably consumed by a `marshalMachinePolymorphicUnion` and used look up the matching decoder machine).
    - **marshalMachinePolymorphicEnvelope** -- like `marshalMachinePolymorphicUnion`, but emits tokens for a map styled like `{kind:typeAbc, msg:{...}}`.

- **obj.Unmarshaller** *struct*

  Top-level control for a system that walks in-memory structures and consumes tokens,
  populating the in-memory fields.  The inverse of `obj.Marshaller`.

  View this like a program stack; `obj.UnmarshalMachine`s are the functions in the stack.

- **UnmarshalMachine** *interface*

  A single state machines that consumes tokens implements this.
  This interface makes the individual machines composable to `Unmarshaller`.

  See the documentation of `MarshalMachine` for more information;
  sinks and sources follow the same model (e.g. these are DFAs, etc).

  - *Implementations:*
    - **unmarshalMachineWildcard** -- populates an `interface{}` (usually with `map[string]interface{}` or `[]interface{}`, for lack of more specific type info).
    - **unmarshalMachineMapWildcard** -- populates a `map[interface{}]interface{}`.
    - **unmarshalMachineMapStringWildcard** -- populates a `map[string]interface{}` (this will yield errors if non-string types are found as map keys).
    - **unmarshalMachineLiteral** -- populates `string`, `int`, etc.
    - **unmarshalMachineStructAtlas** -- uses an `Atlas` to visit fields (presumably all in one structure, but the sky's the limit really since `Atlas` can suggest arbitrary memory locations).
    - **unmarshalMachinePolymorphicUnion** -- expects to consume a single-entry map, and will attempt to shell out to a more specific decoder machine based on the key string.
    - **unmarshalMachinePolymorphicEnvelope** -- like `unmarshalMachinePolymorphicUnion`, but expects to consume two entries: the value of one will be used as a type hint, and the other as the content to send to the specific decoder machine (note this may be significantly less efficient to decode, since contrasted with the union machine, it may require buffering if the type hint entry doesn't come first).
