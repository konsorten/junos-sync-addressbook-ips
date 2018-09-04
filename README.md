# Synchronize IPs into Juniper SRX firewall

This tools is designed to run a Kubernetes Cron Job and keep the Juniper address book updated. A single address-set will be created, including the required IPv4 and IPv6 addresses. All addresses have the address-set name as prefix.

## Environment Variables

The following environment variables are required to run the tool:

| Name | Description | Required? | Example |
| --- | --- | ---| --- |
| JUNIPER_HOST | Name of the DNS name or IP of the Juniper SRX system. | yes | 10.10.10.1 |
| JUNIPER_USER | The name of the user to use for accessing the Juniper. The user is required to have the *super-user* role. | yes | root |
| JUNIPER_PASSWORD | The password of the Juniper user. | yes | **** |
| JUNIPER_ADDRESS_SET | The name of the address-set to create/update. | yes | pingdom-probe-servers |
| IPS_SOURCE_URL | URL to list of IP addresses as plain text document. Multiple URLs can be separated by two semicolons (;;) | yes | https://my.pingdom.com/probes/ipv4 |

## Example URLs

This is a list of popular providers and IP address lists:

| Provider | IPv4 URL | IPv6 URL |
| --- | --- | --- |
| Pingdom | https://my.pingdom.com/probes/ipv4 | https://my.pingdom.com/probes/ipv6 |
| Cloudflare | https://www.cloudflare.com/ips-v4 | https://www.cloudflare.com/ips-v6 |

## Running the Cron Job manually

Issue the following command to create a job from the cron job:

```
kubectl --namespace=juniper-support create job --from=cronjob/junos-sync-addressbook-ips junos-sync-addressbook-ips-manually
```

## Authors

The library is sponsored by the [marvin + konsorten GmbH](http://www.konsorten.de).

We thank all the authors who provided code to this library:

* Felix Kollmann

## License

(The MIT License)

Copyright (c) 2018 marvin + konsorten GmbH (open-source@konsorten.de)

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the 'Software'), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED 'AS IS', WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
