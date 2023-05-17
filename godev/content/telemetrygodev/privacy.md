---
Title: Go Telemetry Privacy Policy
---

# Privacy Policy

*Last updated: April 27, 2023*

When you enable Go toolchain telemetry using `go telemetry on`, Go toolchain programs such as the go command and gopls record usage and performance data about their own execution to local files on your computer stored in `os.UserConfigDir()/go/telemetry`. The files contain event counters, stack traces for the Go toolchain programs, and basic version information about your operating system, CPU architecture, and dependency tools such as the host C compiler and version control tools. The files do not contain any user data that may be potentially identifying or any kind of system identifier.

You can view the locally collected data using `go telemetry view`.

Once a week, the Go toolchain will randomly decide whether to upload that week's reports to a server at Google. The random choice is set so that a representative sample of systems upload reports each week. As more systems participate, each system uploads less often. This data is collected in accordance with the Google Privacy Policy (https://policies.google.com/privacy).
The uploaded reports are republished in full as part of a public dataset. Developers working on Go itself, both inside and outside Google, will use that dataset to better understand how the toolchain is being used and whether it is performing as expected. 

You can collect telemetry information for local viewing without sending it to Google by using `go telemetry local`. If you later switch from `go telemetry local` to `go telemetry on`, the current week’s telemetry data may be uploaded. You may clear your local telemetry data by running `go telemetry clear` at any time.
