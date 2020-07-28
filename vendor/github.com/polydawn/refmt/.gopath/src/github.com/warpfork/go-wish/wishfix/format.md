wishfix file format
===================

The `wishfix` file format is designed to be convenient for human-readable,
human-writable multi-line data.  In particular, it was designed with
storing test fixture data in mind as a primary use-case.

tl;dr:

1. A file contains sections.
2. Each section has a title, a body, and optionally comments.
3. Sections are separated by "---" on their own line.
4. Titles are prefixed by "# " and follow a section separator.
5. Comments (if present) are prefixed by "## " and follow a title.
6. Body is indented with one tab, comes after title (or comments, if present),
   and flanked on both sides by one empty line break for visual clarity.

`wishfix` files are easy to edit.  Adding another blob of content in a new
section requires minimal typing, and importantly, *no escaping*.
Just intent one tab at the front of every line of the body, and you're done.
This should be a dozen keystrokes or less in any text editor.

The typical interface to a `wishfile` is `GetSection(title) -> body`.
(For many use-cases, this is enough; if you're writing test fixtures, you
probably just expect a finite number of well known sections.)
You can also list sections: `GetSections() -> [title]`.
Lastly, by convention we treat the very first section in the file and its
title slightly specially: this first title -- the first line of the file --
can be accessed with `GetMagic() -> string`; this may be useful if you
store a bunch of stuff in `wishfix` files and want to see quickly what it is.


Example
-------

```
# file header

---
# section foobar

	{
		"woo": "zow",
		"indentation": "obviously preserved",
		"json": ["not special"]
	}

---
# section baz
## this will be a comment

	it's all just
	like, free text
	maaaan

---
```



FAQ
---

### What a weird config format.

It's not a config format.  Also, that's not a question.

The `wishfix` format is designed to store a bunch of hunks of data, joined
together in one human-readable file.  This is slightly different than
intending to be used as a general-purpose config or serialization format;
for example, `wishfix` files are supposed to *look good*... and we don't
care if they don't look as good when nested, because that's not a thing.


### Doesn't this look oddly like markdown?

What a coincidence!

Actually.  That was an accident.  I swear.

But it's kinda conveniently true, too.  Previewing a `wishfix` file as markdown
will draw nice little lines between all the sections, and all the section
body blob will be rendered as a code block since it's indented.  Neat!


### Is it binary safe?

Almost.  Your content can't *start* with tab characters.
(Maybe we'll fix this mild oversight sometime if anyone ever actually cares.)
Otherwise, you're good.

Section names are restricted to single lines, and trimmed, however.
Section comments can be multi-line, but are also trimmed on each line.

You should still be slightly wary about assuming on whitespace at end-of-line
will be maintained -- the tools will do it!  But other humans with their pesky
"helpful" text editors these days often have trim-end-of-line-whitespace on
as default behaviors (e.g. Atom does this by default), so if you depend on
trailing whitespace, be prepared to swat coworkers, er, ^W^W bugs all day.


## Does it `git diff` well?

I'm glad you asked!  ABSOLUTELY -- diffing over time was a primary design
consideration.

When loading a `wishfix` file, modifying it, and writing it back out,
everything you didn't change explicitly won't change.

This means *yes*, you can programmatically replace "section 3" of your file
and write it back out, and nothing else in the file will show up in `git diff`.
This is pretty dang useful if you design automatic update systems for your
test fixtures -- you can run the auto-update, and then review diffs at your
leisure!

(Okay, there are some exceptions to this.  The `wishfix` *parser* is slightly
more tolerant than the `wishfix` *marshaller* -- it follows
[Postel's Law](https://en.wikipedia.org/wiki/Postel's_law).  So, if you have
loaded a slightly non-standard file, it may come out slightly more standard.
The now-standard file should round-trip in perpetuity, though.)
