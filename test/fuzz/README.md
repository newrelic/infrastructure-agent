# Fuzz testing

This is a PoC that serves as an example of fuzz testing. 

In this case only agent-integration protocol payload ingest is tested.

This is aimed to be run continuously on CI.

## Usage

### Quick start:

Run all tasks on default target:

> This installs dependencies.

```
make -f test/fuzz/Makefile all
```

Build and run on a given target:

```
TARGET=integration_payload make -f test/fuzz/Makefile build_and_run
TARGET=config_load make -f test/fuzz/Makefile build_and_run
TARGET=logger make -f test/fuzz/Makefile build_and_run
```

See mentioned Makefile for further info.
