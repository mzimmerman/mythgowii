mythgowii
=========

A Go program that uses libcwiid to talk to a wiimote - to be used to control a mythtv frontend like mythpywii

The mainline version of libcwiid doesn't interoperate with cgo due to the way it uses threads so a modified libcwiid needs to be compiled against for usage
The modification is at https://github.com/mzimmerman/cwiid
