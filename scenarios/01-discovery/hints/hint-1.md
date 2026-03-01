# Hint 1: Something Is Missing

You have scanned the network and found 4 open ports. The facility manager said there are roughly
5 to 6 devices. These numbers do not match.

Not every device in an OT facility has an IP address.

Some devices were installed long before Ethernet was common in industrial settings. These devices
communicate over serial buses -- electrical cables, not network cables -- and have no concept of
an IP address. They cannot be reached by ping, and they will not appear in an nmap scan.

If that is true for this facility, how would you reach a device that has no IP address?

Think about what is sitting at port 5030.
