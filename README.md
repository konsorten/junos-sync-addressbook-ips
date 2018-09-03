junos-sync-pingdom-ips

# Running the Cron Job manually

Issue the following command to create a job from the cron job:

```
kubectl --namespace=juniper-support create job --from=cronjob/junos-sync-pingdom-ips junos-sync-pingdom-ips-manually
```
