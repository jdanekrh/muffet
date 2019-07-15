package muffet

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/html"
	"io"
	"mime"
	"strings"
)

type Failures map[string][][]string

func addSuite(fs Failures, url string) {
	if _, ok := fs[url]; !ok {
		fs[url] = make([][]string, 0)
	}
}

func addFailure(fs Failures, url, brokenUrl, error string) {
	fs[url] = append(fs[url], []string{brokenUrl, error})
}

func printFailures(fs Failures) {
	template := `
<?xml version="1.0" encoding="UTF-8" ?> 
   <testsuites id="20140612_170519" name="New_configuration (14/06/12 17:05:19)" tests="225" failures="1262" time="0.001">
      <testsuite id="linkchecker" name="AMQ 7.3 documentation" tests="{{ tests }}" failures="{{ failures }}" time="0.001">
         <testcase id="{{ url }}" name="{{ url }}" time="0.001">
            <failure message="{{ brokenUrl }}" type="ERROR">
WARNING: Use a program name that matches the source file name
Category: COBOL Code Review – Naming Conventions
File: /project/PROGRAM.cbl
Line: 2
      </failure>
    </testcase>
  </testsuite>
</testsuites>
`
	_ = template
}

func mustNot(e error) {
	if e != nil {
		panic(e)
	}
}

func isSinglePageHtmlDocLink(s string) bool {
	return strings.Contains(s, "/html-single/")
}

//^ERROR: 0, ([a-zA-Z:/\.#_?=0-9%&-]+) (.*)
// {"\1", "\2"},
var Whitelist = [][]string{
	// known issues

	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_amq_online_on_openshift_container_platform/#ref-example-roles-messaging", "id #ref-example-roles-messaging not found"},
	//{"https://docs.openshift.org/3.9/creating_images/s2i.html#creating-images-s2i", "id #creating-images-s2i not found"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_amq_console#securing_amq_console_and_amq_broker_connections", "404"},
	//{"http://www.quartz-scheduler.org/documentation/quartz-2.x/tutorials/crontrigger.html", "404"},
	//{"https://github.com/grs/rhea#api", "id #api not found"},
	//{"https://www.apache.org/dist/kafka/2.1.0/RELEASE_NOTES.html", "404"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_amq_console#securing_amq_console_and_amq_broker_connections", "404"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/managing_amq_broker/%7BBrokerManagingBookUrl%7D#upgrading_7.1", "404"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_amq_console/", "404"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/evaluating_amq_online_on_openshift_container_platform/#iot-creating-device-iot", "id #iot-creating-device-iot not found"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/evaluating_amq_online_on_openshift_container_platform/#iot-creating-project-iot", "id #iot-creating-project-iot not found"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/evaluating_amq_online_on_openshift_container_platform/#installing-services-iot", "id #installing-services-iot not found"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/evaluating_amq_online_on_openshift_container_platform/#con-address-space-messaging", "id #con-address-space-messaging not found"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/configuring_amq_broker/#cluster_connections", "id #cluster_connections not found"},
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/configuring_amq_broker/#clustering", "id #clustering not found"},
	//
	////2019/07/01 10:39:51 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_amq_streams_on_red_hat_enterprise_linux_rhel/
	////ERROR: https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_amq_streams_on_red_hat_enterprise_linux_rhel/, 0, https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/amq_streams_1.1_on_red_hat_enterprise_linux_rhel_release_notes 404
	//
	////2019/06/30 21:15:27 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_the_amq_python_client/
	//{"http://qpid.apache.org/releases/qpid-proton-0.28.0/proton/python/api/proton.handlers.MessagingHandler-class.html", "404"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_the_amq_python_client/
	////2019/06/30 21:18:55 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/installing_and_managing_amq_online_on_openshift_container_platform/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/installing_and_managing_amq_online_on_openshift_container_platform/#ref-resources-table-messaging-tenant-messaging", "id #ref-resources-table-messaging-tenant-messaging not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/installing_and_managing_amq_online_on_openshift_container_platform/
	////2019/06/30 21:33:34 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_amq_interconnect/
	//{"https://qpid.apache.org/releases/qpid-dispatch-1.6.0/man/qdstat.html#_qdstat_autolinks", "id #_qdstat_autolinks not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.3/html-single/using_amq_interconnect/
	//
	////2019/06/30 21:52:06 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/amq_clients_overview/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_the_jms_pool_library/", "404"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/amq_clients_overview/
	////2019/06/30 21:53:57 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_the_amq_python_client/
	//{"http://qpid.apache.org/releases/qpid-proton-0.27.0/proton/python/api/proton.handlers.MessagingHandler-class.html", "404"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_the_amq_python_client/
	////2019/06/30 21:56:19 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_the_amq_spring_boot_starter/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_the_jms_pool_library/", "404"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_the_amq_spring_boot_starter/
	////2019/06/30 21:58:16 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_amq_streams_on_red_hat_enterprise_linux_rhel/
	//{"http://kafka.apache.org/documentation/#compaction", "id #compaction not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_amq_streams_on_red_hat_enterprise_linux_rhel/
	////2019/06/30 22:00:09 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/managing_amq_broker/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/getting_started_with_amq_broker/#creating_a_broker_instance", "id #creating_a_broker_instance not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/managing_amq_broker/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/getting_started_with_amq_broker/#installing_on_linux", "id #installing_on_linux not found"},               // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/managing_amq_broker/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/getting_started_with_amq_broker/#download_archive", "id #download_archive not found"},                     // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/managing_amq_broker/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/getting_started_with_amq_broker/#installing_on_windows", "id #installing_on_windows not found"},           // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/managing_amq_broker/
	////2019/06/30 22:03:07 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_amq_online_on_openshift_container_platform/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_amq_online_on_openshift_container_platform/#ref-example-roles-messaging", "id #ref-example-roles-messaging not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/using_amq_online_on_openshift_container_platform/
	////2019/06/30 22:03:47 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/evaluating_amq_online_on_openshift_container_platform/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/evaluating_amq_online_on_openshift_container_platform/#installing-services-iot", "id #installing-services-iot not found"},         // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/evaluating_amq_online_on_openshift_container_platform/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/evaluating_amq_online_on_openshift_container_platform/#con-address-space-messaging", "id #con-address-space-messaging not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/evaluating_amq_online_on_openshift_container_platform/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/evaluating_amq_online_on_openshift_container_platform/#iot-creating-project-iot", "id #iot-creating-project-iot not found"},       // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/evaluating_amq_online_on_openshift_container_platform/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/evaluating_amq_online_on_openshift_container_platform/#iot-creating-device-iot", "id #iot-creating-device-iot not found"},         // https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/evaluating_amq_online_on_openshift_container_platform/
	//
	////2019/06/30 22:05:45 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/
	//{"https://qpid.apache.org/releases/qpid-dispatch-1.0.1/man/qdstat.html#_qdstat_autolinks", "id #_qdstat_autolinks not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/
	////2019/06/30 22:11:53 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/migrating_to_red_hat_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_broker/#acceptor_connector_params%5D", "id #acceptor_connector_params] not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/migrating_to_red_hat_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_broker/configuring_a_point_to_point_using_two_queues", "404"},                      // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/migrating_to_red_hat_amq_7/
	////2019/06/30 22:15:59 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_broker/
	//{"http://search.maven.org/#browse", "id #browse not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_broker/
	////2019/06/30 22:17:11 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/#setting_up_ssl_for_encryption_and_authentication", "id #setting_up_ssl_for_encryption_and_authentication not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/#configure_logging_modules", "id #configure_logging_modules not found"},                                               // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/#router_configuration_reference", "id #router_configuration_reference not found"},                                     // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/#router_network_connections", "id #router_network_connections not found"},                                             // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/#setting_up_message_routing", "id #setting_up_message_routing not found"},                                             // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/#using_router_logs", "id #using_router_logs not found"},                                                               // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/#setting_up_link_routing", "id #setting_up_link_routing not found"},                                                   // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/#configuring_waypoints_autolinks", "id #configuring_waypoints_autolinks not found"},                                   // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_interconnect/#logging_modules_you_can_configure", "id #logging_modules_you_can_configure not found"},                               // https://access.redhat.com/documentation/en-us/red_hat_amq/7.1/html-single/using_amq_console/
	//
	////2019/06/30 22:18:34 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_interconnect/#configure_logging_modules", "id #configure_logging_modules not found"},             // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_interconnect/#configuring_waypoints_autolinks", "id #configuring_waypoints_autolinks not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_interconnect/#setting_up_message_routing", "id #setting_up_message_routing not found"},           // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_console/
	//{"http://localhost:8161/hawtio/login", "dial tcp4 127.0.0.1:8161: connect: connection refused"},                                                                                              // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_interconnect/#using_router_logs", "id #using_router_logs not found"},                             // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_console/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_interconnect/#setting_up_link_routing", "id #setting_up_link_routing not found"},                 // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_console/
	////2019/06/30 22:19:36 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_broker/
	//{"http://search.maven.org/#browse", "id #browse not found"},                                                                           // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_broker/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_broker/configure_delayed_redelivery", "404"},    // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_broker/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_broker/configure_dead_letter_addresses", "404"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_broker/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_broker/set_message_expiry", "404"},              // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_broker/
	////2019/06/30 22:24:39 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/introducing_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_the_amq_c%2B%2B_client/", "404"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/introducing_red_hat_jboss_amq_7/
	////2019/06/30 22:26:03 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_interconnect/
	//{"https://qpid.apache.org/releases/qpid-dispatch-0.8.0/man/qdstat.html#_qdstat_autolinks", "id #_qdstat_autolinks not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/using_amq_interconnect/
	////2019/06/30 22:26:56 fetching https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#configuring_an_address_for_publish_subscribe_messaging", "id #configuring_an_address_for_publish_subscribe_messaging not found"},                         // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#configuring_two_way_tls", "id #configuring_two_way_tls not found"},                                                                                       // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#ldap_authn", "id #ldap_authn not found"},                                                                                                                 // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#configuring_a_point_to_point_address_with_two_queues", "id #configuring_a_point_to_point_address_with_two_queues not found"},                             // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#configure_destinations_artemis", "id #configure_destinations_artemis not found"},                                                                         // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#handling_slow_messaging_consumers", "id #handling_slow_messaging_consumers not found"},                                                                   // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/%7Bbook_link%7D#configuring_network_access", "404"},                                                                                             // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#acceptor_connector_params%5D", "id #acceptor_connector_params] not found"},                                                                               // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#guest_auth", "id #guest_auth not found"},                                                                                                                 // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#configuring_one_way_tls", "id #configuring_one_way_tls not found"},                                                                                       // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#configuring_authorization", "id #configuring_authorization not found"},                                                                                   // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#cert_authn", "id #cert_authn not found"},                                                                                                                 // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#configuring_an_address_to_use_point_to_point_and_publish_subscribe", "id #configuring_an_address_to_use_point_to_point_and_publish_subscribe not found"}, // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	//{"https://access.redhat.com/documentation/en-us/red_hat_jboss_amq/7.0/html-single/using_amq_broker/#configuring_an_address_for_point_to_point_messaging", "id #configuring_an_address_for_point_to_point_messaging not found"},                               // https://access.redhat.com/documentation/en-us/red_hat_amq/7.0/html-single/migrating_to_red_hat_jboss_amq_7/
	////2019/06/30 23:01:25 fetching https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/integrating_with_jboss_enterprise_application_platform/
	//{"http://localhost:8080/jboss-helloworld-mdb/HelloWorldMDBServletClient?topic", "dial tcp4 127.0.0.1:8080: connect: connection refused"}, // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/integrating_with_jboss_enterprise_application_platform/
	//{"http://localhost:8080/jboss-helloworld-mdb/HelloWorldMDBServletClient", "dial tcp4 127.0.0.1:8080: connect: connection refused"},       // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/integrating_with_jboss_enterprise_application_platform/
	////2019/06/30 23:02:13 fetching https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/using_networks_of_brokers/
	//{"https://access.redhat.com/site/documentation/JBoss_Fuse/", "404"}, // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/using_networks_of_brokers/
	////2019/06/30 23:03:03 fetching https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/tuning_guide/
	//{"http://en.wikipedia.org/wiki/Network_Improvement", "404"}, // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/tuning_guide/
	////2019/06/30 23:05:06 fetching https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/red_hat_jboss_a-mq_for_openshift/
	////--- FAIL: TestNewChecker2 (7036.74s)
	////doccheck.go:69:
	////Error Trace:    doccheck.go:69
	////Error:          Not equal:
	////expected: 200
	////actual  : 0
	////Test:           TestNewChecker2
	////doccheck.go:70:
	////Error Trace:    doccheck.go:70
	////Error:          Should be true
	////Test:           TestNewChecker2
	////doccheck.go:71:
	////Error Trace:    doccheck.go:71
	////Test:           TestNewChecker2
	////panic: runtime error: invalid memory address or nil pointer dereference [recovered]
	////panic: runtime error: invalid memory address or nil pointer dereference
	////[signal SIGSEGV: segmentation violation code=0x1 addr=0x10 pc=0x6df359]
	////
	////	goroutine 6 [running]:
	////	testing.tRunner.func1(0xc0001c4100)
	////	/nix/store/q6l8pwyqn3qkicmhdg2z026m9b593kf5-go-1.11.6/share/go/src/testing/testing.go:792 +0x387
	////	panic(0x73dae0, 0xa41e00)
	////	/nix/store/q6l8pwyqn3qkicmhdg2z026m9b593kf5-go-1.11.6/share/go/src/runtime/panic.go:513 +0x1b9
	////	github.com/raviqqe/muffet.TestNewChecker2(0xc0001c4100)
	////	/home/jdanek/repos/linkchecks/doccheck.go:73 +0xaf9
	////	testing.tRunner(0xc0001c4100, 0x7be740)
	////	/nix/store/q6l8pwyqn3qkicmhdg2z026m9b593kf5-go-1.11.6/share/go/src/testing/testing.go:827 +0xbf
	////	created by testing.(*T).Run
	////	/nix/store/q6l8pwyqn3qkicmhdg2z026m9b593kf5-go-1.11.6/share/go/src/testing/testing.go:878 +0x35c
	//
	////Expected :200
	////Actual   :0
	////<Click to see difference>
	//
	////{"https://v1-9.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.9/#SecretVolumeSource-v1-core", "x509: certificate is valid for *.netlify.com, netlify.com, not v1-9.docs.kubernetes.io"},
	////{"https://v1-9.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.9/#SecretKeySelector-v1-core", "x509: certificate is valid for *.netlify.com, netlify.com, not v1-9.docs.kubernetes.io"},
	////{"https://v1-9.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.9/#podsecuritycontext-v1-core", "x509: certificate is valid for *.netlify.com, netlify.com, not v1-9.docs.kubernetes.io"},
	////{"https://v1-9.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.9/#networkpolicypeer-v1-networking", "x509: certificate is valid for *.netlify.com, netlify.com, not v1-9.docs.kubernetes.io"},
	////{"https://v1-9.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.9/#tolerations-v1-core", "x509: certificate is valid for *.netlify.com, netlify.com, not v1-9.docs.kubernetes.io"},
	////{"https://v1-9.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.9/#localobjectreference-v1-core", "x509: certificate is valid for *.netlify.com, netlify.com, not v1-9.docs.kubernetes.io"},
	////{"https://v1-9.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.9/#ConfigMapVolumeSource-v1-core", "x509: certificate is valid for *.netlify.com, netlify.com, not v1-9.docs.kubernetes.io"},
	////{"https://v1-9.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.9/#affinity-v1-core", "x509: certificate is valid for *.netlify.com, netlify.com, not v1-9.docs.kubernetes.io"},
	////{"https://v1-9.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.9/#ConfigMapKeySelector-v1-core", "x509: certificate is valid for *.netlify.com, netlify.com, not v1-9.docs.kubernetes.io"},

	{"http://localhost:8161", "dial tcp4 127.0.0.1:8161: connect: connection refused"},
	{"http://localhost:8161/console/login", "dial tcp4 127.0.0.1:8161: connect: connection refused"},
	{"http://localhost:8161/jolokia", "dial tcp4 127.0.0.1:8161: connect: connection refused"},
	{"http://localhost:8161/jolokia/read/org.apache.activemq.artemis:module=Core,type=Server/Version", ""},
	{"https://broker-amq-0.broker-amq-headless.amq-demo.svc", "lookup broker-amq-0.broker-amq-headless.amq-demo.svc: no such host"},
	{"http://broker-amq-0.broker-amq-headless.amq-demo.svc", "lookup broker-amq-0.broker-amq-headless.amq-demo.svc: no such host"},

	{"http://kafka.apache.org/20/documentation.html#connectconfigs", "id #connectconfigs not found"},
	{"http://kafka.apache.org/20/documentation.html#producerconfigs", "id #producerconfigs not found"},
	{"http://kafka.apache.org/20/documentation.html#brokerconfigs", "id #brokerconfigs not found"},
	{"http://kafka.apache.org/20/documentation.html#newconsumerconfigs", "id #newconsumerconfigs not found"},
	// todo: url does not have /20/?
	{"http://kafka.apache.org/documentation/#security_authz", "id #security_authz not found"},
	{"http://kafka.apache.org/documentation/#brokerconfigs", "id #brokerconfigs not found"},
	{"http://kafka.apache.org/documentation/#compaction", "id #connectconfigs not found"},

	{"https://access.redhat.com/containers/#/product/RedHatAmq", "id #/product/RedHatAmq not found"},

	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-messaging-v1.0-os.html#section-message-format", "id #section-message-format not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-messaging-v1.0-os.html#type-amqp-sequence", "id #type-amqp-sequence not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-messaging-v1.0-os.html#type-amqp-value", "id #type-amqp-value not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-messaging-v1.0-os.html#type-application-properties", "id #type-application-properties not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-messaging-v1.0-os.html#type-data", "id #type-data not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-messaging-v1.0-os.html#type-properties", "id #type-properties not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#toc", "id #toc not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-array", "id #type-array not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-binary", "id #type-binary not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-boolean", "id #type-boolean not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-byte", "id #type-byte not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-char", "id #type-char not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-double", "id #type-double not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-float", "id #type-float not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-int", "id #type-int not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-list", "id #type-list not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-long", "id #type-long not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-map", "id #type-map not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-null", "id #type-null not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-short", "id #type-short not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-string", "id #type-string not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-symbol", "id #type-symbol not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-timestamp", "id #type-timestamp not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-ubyte", "id #type-ubyte not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-uint", "id #type-uint not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-ulong", "id #type-ulong not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-ushort", "id #type-ushort not found"},
	//{"http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-types-v1.0-os.html#type-uuid", "id #type-uuid not found"},

	// outdated
	// https://access.redhat.com/containers/?application_categories_list=Messaging#/search/online
	// https://access.redhat.com/containers/?product=Red%20Hat%20AMQ&application_categories_list=Messaging#/search/online
	//{"https://access.redhat.com/containers/?/product=Red%20Hat%20AMQ&application_categories_list=Messaging#/search/online", "id #/search/online not found"},

	{"https://access.redhat.com/labs/#?type=config", "id #?type=config not found"},
	{"https://access.redhat.com/labs/#?type=deploy", "id #?type=deploy not found"},
	{"https://access.redhat.com/labs/#?type=security", "id #?type=security not found"},
	{"https://access.redhat.com/labs/#?type=troubleshoot", "id #?type=troubleshoot not found"},
	{"https://access.redhat.com/management/subscriptions/#active", "id #active not found"},
	{"https://access.redhat.com/security/security-updates/#/cve", "id #/cve not found"},
	{"https://access.redhat.com/security/security-updates/#/security-advisories", "id #/security-advisories not found"},
	{"https://access.redhat.com/security/security-updates/#/security-labs", "id #/security-labs not found"},

	//{"https://access.redhat.com/articles/3824851", "dialing to the given TCP address timed out"},

	//{"https://issues.jboss.org/browse/ENTESB-3155", "dial tcp4 209.132.182.82:443: connect: no route to host"},    // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/release_notes/
	//{"https://issues.jboss.org/browse/ENTESB-5761", "dial tcp4 209.132.182.82:443: connect: no route to host"},    // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/release_notes/
	//{"https://issues.jboss.org/browse/ENTESB-5911", "dial tcp4 209.132.182.82:443: connect: no route to host"},    // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/release_notes/
	//{"https://issues.jboss.org/browse/ENTESB-5647", "dial tcp4 209.132.182.82:443: connect: no route to host"},    // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/release_notes/
	//{"https://issues.apache.org/jira/browse/AMQ-6256", "dial tcp4 207.244.88.139:443: connect: no route to host"}, // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/release_notes/
	//{"https://issues.jboss.org/browse/ENTESB-4012", "dial tcp4 209.132.182.82:443: connect: no route to host"},    // https://access.redhat.com/documentation/en-us/red_hat_jboss_a-mq/6.3/html-single/release_notes/
	//{"https://issues.jboss.org/browse/ENTMQBR-2020", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/browse/ENTMQIC-2149", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-2011", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-897", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-1466", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-1018", "timeout"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-636", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-1500", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-956", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-1061", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-1848", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/browse/ENTMQBR-1498", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/browse/ENTMQBR-1783", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-652", "dialing to the given TCP address timed out"},
	//{"https://access.redhat.com/solutions/3269061", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-1995", "dialing to the given TCP address timed out"},
	//{"https://issues.jboss.org/jira/browse/ENTMQBR-1045", "dialing to the given TCP address timed out"},

	// stage

	{"https://access.stage.redhat.com/ecosystem/search/#/ecosystem", ""},
	{"https://doc-stage.usersys.redhat.com/solution-engine", ""},
	{"https://access.stage.redhat.com/insights/?intcmp=mm|t|c1|rhaidec2015&", ""},
	{"https://access.stage.redhat.com/insights/info/?intcmp=mm|p|im|rhaijan2016&", ""},
	{"https://access.stage.redhat.com/insights/info/?intcmp=mm|t|c1|rhaidec2015&", ""},
	{"https://access.stage.redhat.com/security/security-updates/#/cve", ""},
	{"https://access.stage.redhat.com/products/red-hat-certificate-system/", "timeout"},
	{"https://access.stage.redhat.com/management/subscriptions/#active", ""},

	{"https://access.stage.redhat.com/changeLanguage?language=pt", ""},
	{"https://access.stage.redhat.com/changeLanguage?language=zh_CN", ""},
	{"https://access.stage.redhat.com/changeLanguage?language=fr", ""},
	{"https://access.stage.redhat.com/changeLanguage?language=en", ""},
	{"https://access.stage.redhat.com/changeLanguage?language=de", ""},
	{"https://access.stage.redhat.com/changeLanguage?language=ko", ""},
	{"https://access.stage.redhat.com/changeLanguage?language=ru", ""},
	{"https://access.stage.redhat.com/changeLanguage?language=es", ""},
	{"https://access.stage.redhat.com/changeLanguage?language=it", ""},
	{"https://access.stage.redhat.com/changeLanguage?language=ja", ""},

	{"https://access.stage.redhat.com/security/security-updates/#/security-labs", ""},
	{"https://access.stage.redhat.com/security/security-updates/#/security-advisories", ""},
	{"https://access.stage.redhat.com/support/cases/", ""},
	{"https://www.stage.redhat.com/wapps/ugc/register.html", ""},
}

func fetchVersions(f fetcher, u string) (versions []string, err error) {
	versions = make([]string, 0)

	f.connectionSemaphore.Request()
	defer f.connectionSemaphore.Release()

	req, res := fasthttp.Request{}, fasthttp.Response{}
	req.SetRequestURI(u)
	req.SetConnectionClose()

	for k, v := range f.options.Headers {
		req.Header.Add(k, v)
	}

	r := 0

redirects:
	for {
		err = f.client.DoTimeout(&req, &res, f.options.Timeout)

		if err != nil {
			return
		}

		switch res.StatusCode() / 100 {
		case 2:
			break redirects
		case 3:
			r++

			if r > f.options.MaxRedirections {
				err = errors.New("too many redirections")
				return
			}

			bs := res.Header.Peek("Location")

			if len(bs) == 0 {
				err = errors.New("location header not found")
				return
			}

			req.URI().UpdateBytes(bs)
		default:
			err = fmt.Errorf("%v", res.StatusCode())
			return
		}
	}

	if s := strings.TrimSpace(string(res.Header.Peek("Content-Type"))); s != "" {
		var t string
		t, _, err = mime.ParseMediaType(s)

		if err != nil {
			return
		} else if t != "text/html" {
			err = errors.New("not text/html")
			return
		}
	}

	z := html.NewTokenizer(bytes.NewReader(res.Body()))
	for {
		if z.Next() == html.ErrorToken {
			err = z.Err()
			if err == io.EOF {
				err = nil
			}
			return
		}
		t := z.Token()
		switch t.Type {
		case html.StartTagToken, html.SelfClosingTagToken:
			var class string
			var value string
			//name, hasAttr := z.TagName()
			//log.Printf("%v,  %v, %#v\n", name, hasAttr, t)
			if t.Data != "input" {
				continue
			}
			for _, a := range t.Attr {
				if a.Key == "class" {
					class = a.Val
				}
				if a.Key == "value" {
					value = a.Val
				}
			}
			if strings.Contains(class, "versionFilter") {
				versions = append(versions, value)
				break
			}
		}
	}
}
