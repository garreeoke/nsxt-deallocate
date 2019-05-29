# NSX-T Trace #
Use NSX-T trace api for tracing between containers

### Run on a linux box ###

* Download the nsxt-trace-linux.tgz from the artifacts directory.
* tar xzvf ./nsxt-trace-linux.tgz
* chmod +x nsxt-trace
* mv nsxt-trace /usr/local/bin (or somewhere in PATH)

### Arguments ###

_Required_
* --nsx
    * ip/fqdn of NSX Manager
    * Can set NSX_MANAGER env variable as an alternative
* --nsx-pass
    * password for NSX user
    * Set NSX_PASS env variable as an alternative
* --namespace
    * kubernetes namespace where pods live
* --src-pod
    * name of the source pod
* --src-port
    * port of the source pod
* --dst-pod
    * name of the destination pod
* --dst-port
    * port of the destination pod
    
_Optional_
* --nsx-user
    * user for nsx manager, admin is default
    * Set NSX_USER env variable as an alternative
* --dst-namespace
    * specify a different namespace for the destination
* --payload
    * send a text payload
    
### Examples ###

* nsxt_trace --nsx-ip=10.173.61.44 --nsx-user=admin --nsx-pass=VMware1! --namespace=acme-air --src-pod=acme-web-c6bbf95d5-wlrwf --src-port=8080 --dst-pod=mongodb-0 --dst-port=27017
* nsxt_trace --nsx-ip=10.173.61.44 --nsx-user=admin --nsx-pass=VMware1! --namespace=acme-air --src-pod=acme-web-c6bbf95d5-wlrwf --src-port=8080 --dst-pod=mongodb-0 --dst-port=27017 --dst-namespace=acme-air2