# NSX-T Deallocate #
Clean up orphaned NSX-T IP addresses from a VIP pool.

USE AT YOUR OWN RISK!

### Run on a linux box ###

* Download the nsxt-deallocate-linux.tgz from the artifacts directory.
* tar xzvf ./nsxt-deallocate-linux.tgz

### Arguments ###

_Required_
* --t0 
    * uuid of the t0 router to check nat rules on
* --pool-id
    * uuid of the IP pool to check for addresses
* --nsx-ip 
    * ip address of nsx-t manager
* --nsx-pass
    * password of nsx manager user
    
_Optional_
* --nsx-user
    * user for nsx manager, admin is default
* --delete
    * use true if you want to execute the delete
    
### Example ###

 ./nsxt-deallocate --t0-id=5f79535c-8261-4b3b-a60e-ca2878657c39 --pool-id=4672673b-4edc-48fa-9ad5-d5389d1e0275 --nsx-ip=10.173.61.44 --nsx-user=admin --nsx-pass=VMware1! --delete=true 
