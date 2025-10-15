│ --version      Show the version and exit.                                                                                                                           │
│ --help         Show this message and exit.                                                                                                                          │
╰─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
╭─ Instance management ───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ create       Create a new Thunder Compute instance.                                                                                                                 │
│ delete       Permanently delete a Thunder Compute instance. This action is not reversible                                                                           │
│ start        Start a stopped Thunder Compute instance. All data in the persistent storage will be preserved                                                         │
│ stop         Stop a running Thunder Compute instance. Stopped instances have persistent storage and can be restarted at any time                                    │
│ modify       Modify a Thunder Compute instance's properties (CPU, GPU, storage, mode)                                                                               │
│ snapshot     Manage instance snapshots: create, list, or delete.                                                                                                    │
╰─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
╭─ Utility ───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ connect          Connect to the Thunder Compute instance with the specified instance_id                                                                             │
│ status           List details of Thunder Compute instances within your account                                                                                      │
│ scp              Transfers files between your local machine and Thunder Compute instances.                                                                          │
│ update           Check for and apply updates to the Thunder CLI.                                                                                                    │
╰─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
╭─ Account management ────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ login    Log in to Thunder Compute, prompting the user to generate an API token at console.thundercompute.com. Saves the API token to ~/.thunder/token              │
│ logout   Log out of Thunder Compute and deletes the saved API token 