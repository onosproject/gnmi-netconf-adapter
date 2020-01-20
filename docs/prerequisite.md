# Prerequisites required for gnmi-netconf-adapter

Please ensure that your are familiar with the developer workflow:

- [Developer Workflow](https://docs.onosproject.org/developers/dev_workflow/)

## Overriding the build variables

There are two options here;

- update the [.env file](../.env) with static values that are specific to your project
- use the --environment-overrides flag in make e.g. PRJ_VERSION=overridden make build --environment-overrides
