## bulb - Is not stem
### Yawning Angel (yawning at torproject dot org)

bulb is a Go language interface to the Tor control port.  It is considerably
lighter in functionality than stem and other controller libraries, and is
intended to be used in combination with`control-spec.txt`.

It was written primarily as a not-invented-here hack, and the base interface is
more than likely to stay fairly low level, though useful helpers will be added
as I need them.

Things you should probably use instead:
 * [stem](https://stem.torproject.org)
 * [txtorcon](https://pypi.python.org/pypi/txtorcon)
 * [orc](https://github.com/sycamoreone/orc)

Bugs:
 * bulb does not send the 'QUIT' command before closing the connection.
