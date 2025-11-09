# WOL-Shutdown-Service
A simple windows service to listen for WOL packets and shutdown your PC.

# Installing, Uninstalling, etc.
Use the ``setup-service.bat`` file.

# Compiling
Install Go, then run the ``build.bat`` file.

**I won't provide an EXE file.** Sorry.

# Configs
Inside the ``config.ini`` you'll find the MAC address field that should contain the MAC address that the packet is targetting (yours in this case, depending on what interface your using).

You'll also find a blank IP allowlist, which if empty will allow packets from any IP, filling that means you'll only accept packets from that/those IP(s).

**Please note that this file is required** for the service to start.

# Logging
This service registers a logging section in the Windows event logs, so you can watch there what it's doing and also the logs for when it does something or errors out due to a missing config file.

# Contributing
Fork and do it there. I'm not interested ATM in contribs.

# License
Check the LICENSE file.