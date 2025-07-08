# Building New Relic Infrastructure agent on windows.
## Requirements
1. [Go](https://golang.org/dl/)
2. [MSBuild](https://docs.microsoft.com/en-us/visualstudio/msbuild/msbuild?view=vs-2022)
3. [Wix Toolset](https://wixtoolset.org/)

## Compile and build the agent msi package

```bash
    # Run the unit tests
    .\test.ps1

    # Build the binaries, supported architectures: [amd64, 386]
    .\build.ps1 -skipSigning -arch amd64

    # Create the package
    .\package_msi.ps1 -skipSigning -arch amd64
```

The packages will be stored in "dist" directory from project root.