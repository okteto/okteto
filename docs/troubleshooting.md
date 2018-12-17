# Troubleshooting

## Files are not syncing
cnd uses  [syncthing](https://docs.syncthing.ne) to sync files between your environments. If your cloud native environment is not being updated correctly, review the following:

1. The `cnd up` process is running
1. Verify that syncthing is running on your environment (there should be two processes per cnd environment running)
1. Rerun `cnd up` (give it a few minutes to reestablish synchronization)

## Files syncing is slow
Please follow [syncthing's docs](https://docs.syncthing.net/users/faq.html#why-is-the-sync-so-slow) to troubleshoot this.

