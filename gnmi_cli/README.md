The GNMI CLI may be used to demo the `gnmi-netconf-adapter`. Example files in this directory. 

Typical usage is as follows:

````
gnmi_cli  \
-address  localhost:10999   \
-client_key  ../pkg/certs/client1.key  \
-client_crt  ../pkg/certs/client1.crt    \
-ca_crt ../pkg/certs/onfca.crt  \
-set -proto "$(cat set.version.gnmi)"
````