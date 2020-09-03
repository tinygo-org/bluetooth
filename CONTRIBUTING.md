# How to contribute

Thank you for your interest in improving the Go Bluetooth module.

We would like your help to make this project better, so we appreciate any contributions. See if one of the following descriptions matches your situation:

### New to Bluetooth Programming

We'd love to get your feedback on getting started with the Go Bluetooth. Run into any difficulty, confusion, or anything else? You are not alone. We want to know about your experience, so we can help the next people. Please open a Github issue with your questions, or you can also get in touch directly with us on our Slack channel at [https://gophers.slack.com/messages/CDJD3SUP6](https://gophers.slack.com/messages/CDJD3SUP6).

### Something in the Go Bluetooth package is not working as you expect

Please open a Github issue with your problem, and we will be happy to assist.

### Something related to Bluetooth programming that you want/need does not appear to be in Go Bluetooth

We probably have not implemented it yet. Your pull request adding the functionality would be greatly appreciated.

Please open a Github issue. We want to help, and also make sure that there is no duplications of efforts. Sometimes what you need is already being worked on by someone else.

## How to use our Github repository

The `release` branch of this repo will always have the latest released version of the Go Bluetooth module. All of the active development work for the next release will take place in the `dev` branch. The Go Bluetooth module will use semantic versioning and will create a tag/release for each release.

Here is how to contribute back some code or documentation:

- Fork repo
- Create a feature branch off of the `dev` branch
- Make some useful change
- Make sure the tests still pass
- Submit a pull request against the `dev` branch.
- Be kind

## How to run tests

To run the bare metal tests:

```
make smoketest-tinygo
```

To run tests for a specific operating system:

```
make smoketest-linux
```

or

```
make smoketest-macos
```

or

```
make smoketest-windows
```

You should be able to run the tests for your own operating system. Note that cross-compilation may or may not work, depending on which tools you have installed.
