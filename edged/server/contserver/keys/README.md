## This folder contains the private key and cert for `*.cdn.beta.gladiuspool.com`

Normally, this would be a bad idea, but because the service worker does hash validation of the files we don't have to wory about trusting the content nodes. Also, this cert is only valid for the `*.cdn.beta.gladiuspool.com` domain and can't be used to impersonate `gladiuspool.com`