# smartedge
A new way to integrate cloud, sensors, actuators, and all types of devices and services on a smart edge device.

The SmartEdge service allows any IoT device, cloud, sensor or similar to be easily integrated.

The concept is simple. Use in for incoming data. Out for outgoing data. Wrap it into MQTT. Now integrate anything that generates data or can accept actions or data.

In: Wrap any service that gets data via a myservice.in and either read the data from an executable that gets polled or from an mqtt topic at in/myservice (call inmqtt to get the mqtt configuration).
Out: Listen to the MQTT topic out/myservice and call myservice.out with any data.
Init: Initialize your service for the first time.
Config: (Re)Configure your service

Basic concepts, you need to create 4 exectuables that accept a JSON file as input:
   (myservice).in - command to get data from myservice
   (myservice).out - command to get myservice to do an action or send data
   (myservice).init - intialize the service - called 1 time
   (myservice).config - configure the service

 The (myservice) can be wrapped inside an MQTT subscriber with the topic in/(myservice) and invoke (myservice).in
 or alternatively call smartedge inmqtt to get the JSON configuration to connect to the MQTT server to pass data.

 The (myservice) can listen for actions on the MQTT topic out/(myservice). Just start the service with smartedge outmqtt.

 To configure the initial service call:
    smartedge config --dir "where config is stored" \
        --transport "{Port: 1833, Host: localhost, ClientID: myservice, UserName: user, Password: pass, Certificate: ./cacert.crt}" \
        --init "{myservicejson:toinit}" \
        --conf "{myservicejson:to(re)conf}"

If you want to rename myservice to something else just change the myservice executables in the top directory and make sure your ClientID reflects the new name as well.
