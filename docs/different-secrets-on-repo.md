# Different secrets/auth for actions.

For each created webhook github sends a 
[ping request](https://docs.github.com/en/webhooks/webhook-events-and-payloads#ping).

This request is intended to verify, that the setup is working correctly.

The problem lies in the fact, that github doesn't allow to specify a branch, for
which the request is triggered, and branch is a required part for us to run 
actions on.

Because of that, we can't determine what action should be triggered on the ping
event, only which project. At the same time, you can absolutely setup
webhooks with different secrets on the same project, but then we have no way of
actually verifying them.

To circumvent this behavior, we don't allow secrets and auth info to be setup
on the action level, only on the project level itself.

If you need to have multiple different secrets for the same repo on different 
actions, you must create different projects in the config for them.