# Testing

The tests implemented perform *functional* tests of the library. This means that 
the tests performed interact with a GPIO chipset, and perform actual read/write 
operations. Using this test set, it's possible to quickly and accurately check 
if the library is working as expected on a specific hardware/kernel combination.

## Requirements

Although the library is not Raspberry Pi specific, the GPIO pin names used for 
tests are.

As written, the tests must be executed on a Raspberry Pi SBC running Linux. Tested 
models are:

* Raspberry Pi 3B
* Raspberry Pi Zero W
* Raspberry Pi 4
* Raspberry Pi 5

You must also have the golang SDK installed.

## Setting Up

In order to execute the functional tests, you must jumper the sets of pins shown 
below together.

For example, the single line tests require GPIO5 and GPIO13 to be connected to 
each other, so a jumper is required between pins 29 and 33. For the multi-line 
tests to work, you must connect the following GPIO pins together with jumpers.

| GPIO Output | Output Pin # | GPIO Input | Input Pin # |
| ----------- | ------------ | ---------- | ----------- |
| GPIO2       |            3 | GPIO10     |          19 |
| GPIO3       |            5 | GPIO11     |          23 |
| GPIO4       |            7 | GPIO12     |          32 |
| GPIO5       |           29 | GPIO13     |          33 |
| GPIO6       |           31 | GPIO14     |           8 |
| GPIO7       |           26 | GPIO15     |          10 |
| GPIO8       |           24 | GPIO16     |          36 |
| GPIO9       |           21 | GPIO17     |          11 |

## Cross-Compiling
If you don't have a working go installation on the target machine, you can cross
compile from one machine and then copy the test binary to the target machine.

To cross compile for Raspberry Pi, execute the command:

```bash
$periph.io/x/host/gpioctl> GOOS=linux GOARCH=arm64 go test -c
$periph.io/x/host/gpioctl> scp gpioioctl.test user@test.machine:~
$periph.io/x/host/gpioctl> ssh user@test.machine
$user> ./gpioioctl.test -test.v
```
for Pi Zero W, use:

```bash
$periph.io/x/host/gpioctl> GOOS=linux GOARCH=arm GOARM=6 go test -c
$periph.io/x/host/gpioctl> scp gpioioctl.test user@test.machine:~
$periph.io/x/host/gpioctl> ssh user@test.machine
$user> ./gpioioctl.test -test.v

```

## Executing the Tests

After connecting the jumper wires as shown above, and you have golang installed 
and the go/bin directory in the path, change to this directory and execute the 
command:

```bash
$> go test -v -cover
```
