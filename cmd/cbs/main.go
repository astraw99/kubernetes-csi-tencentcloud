package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/dbdd4us/qcloudapi-sdk-go/metadata"
	"github.com/golang/glog"

	"github.com/tencentcloud/kubernetes-csi-tencentcloud/driver/cbs"
	"github.com/tencentcloud/kubernetes-csi-tencentcloud/driver/util"
)

const (
	TENCENTCLOUD_CBS_API_SECRET_ID  = "TENCENTCLOUD_CBS_API_SECRET_ID"
	TENCENTCLOUD_CBS_API_SECRET_KEY = "TENCENTCLOUD_CBS_API_SECRET_KEY"

	ClusterId = "CLUSTER_ID"
)

var (
	endpoint            = flag.String("endpoint", fmt.Sprintf("unix:///var/lib/kubelet/plugins/%s/csi.sock", cbs.DriverName), "CSI endpoint")
	region              = flag.String("region", "", "tencent cloud api region")
	zone                = flag.String("zone", "", "cvm instance region")
	cbsUrl              = flag.String("cbs_url", "cbs.internal.tencentcloudapi.com", "cbs api domain")
	volumeAttachLimit   = flag.Int64("volume_attach_limit", -1, "Value for the maximum number of volumes attachable for all nodes. If the flag is not specified then the value is default 20.")
	metricsServerEnable = flag.Bool("enable_metrics_server", true, "enable metrics server, set `false` to close it.")
	metricsPort         = flag.Int64("metric_port", 9099, "metric port")
	timeInterval        = flag.Int("time-interval", 60, "the time interval for synchronizing cluster and disks tags, just for test")
	master              = flag.String("master", "", "Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.")
	kubeconfig          = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
)

func main() {
	flag.Parse()
	defer glog.Flush()

	var config *rest.Config
	var err error
	if *master != "" || *kubeconfig != "" {
		glog.Infof("Either master or kubeconfig specified. building kube config from that..")
		config, err = clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	} else {
		glog.Infof("Building kube configs for running in cluster...")
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Fatalf("Failed to create config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create client: %v", err)
	}

	metadataClient := metadata.NewMetaData(http.DefaultClient)

	if *region == "" {
		r, err := util.GetFromMetadata(metadataClient, metadata.REGION)
		if err != nil {
			glog.Fatal(err)
		}
		region = &r
	}
	if *zone == "" {
		z, err := util.GetFromMetadata(metadataClient, metadata.ZONE)
		if err != nil {
			glog.Fatal(err)
		}
		zone = &z
	}

	u, err := url.Parse(*endpoint)
	if err != nil {
		glog.Fatalf("parse endpoint err: %s", err.Error())
	}

	if u.Scheme != "unix" {
		glog.Fatal("only unix socket is supported currently")
	}

	cp := util.NewCachePersister()

	drv, err := cbs.NewDriver(*region, *zone, os.Getenv(ClusterId), *volumeAttachLimit, clientset)
	if err != nil {
		glog.Fatal(err)
	}

	if err := drv.Run(u, *cbsUrl, cp, *metricsServerEnable, *timeInterval, *metricsPort); err != nil {
		glog.Fatal(err)
	}

	return
}
