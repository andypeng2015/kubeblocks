/*
Copyright 2022 The Kubeblocks Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dbaas

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"

	policyv1 "k8s.io/api/policy/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("Cluster Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

	const leader = "leader"
	const follower = "follower"

	clusterObjKey := types.NamespacedName{
		Name:      "my-cluster",
		Namespace: "default",
	}

	checkedCreateObj := func(obj client.Object) error {
		if err := k8sClient.Create(context.Background(), obj); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}

	assureDefaultStorageClassObj := func() *storagev1.StorageClass {
		By("By assure an default storageClass")
		scYAML := `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-hostpath-sc
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: hostpath.csi.k8s.io
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
`
		sc := &storagev1.StorageClass{}
		Expect(yaml.Unmarshal([]byte(scYAML), sc)).Should(Succeed())
		Expect(checkedCreateObj(sc)).Should(Succeed())
		return sc
	}

	assureCfgTplConfigMapObj := func(cmName string) *corev1.ConfigMap {
		By("By assure an cm obj")
		appVerYAML := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: mysql-tree-node-template-8.0
  namespace: default
data:
  my.cnf: |-
    [mysqld]
    innodb-buffer-pool-size=512M
    log-bin=master-bin
    gtid_mode=OFF
    consensus_auto_leader_transfer=ON
    
    log-error=/data/mysql/log/mysqld.err
    character-sets-dir=/usr/share/mysql-8.0/charsets
    datadir=/data/mysql/data
    port=3306
    general_log=1
    general_log_file=/data/mysql/mysqld.log
    pid-file=/data/mysql/run/mysqld.pid
    server-id=1
    slow_query_log=1
    slow_query_log_file=/data/mysql/mysqld-slow.log
    socket=/data/mysql/tmp/mysqld.sock
    ssl-ca=/data/mysql/std_data/cacert.pem
    ssl-cert=/data/mysql/std_data/server-cert.pem
    ssl-key=/data/mysql/std_data/server-key.pem
    tmpdir=/data/mysql/tmp/
    loose-sha256_password_auto_generate_rsa_keys=0
    loose-caching_sha2_password_auto_generate_rsa_keys=0
    secure-file-priv=/data/mysql
    
    [client]
    password=
    user=root
    port=3306
    socket=/data/mysql/tmp/mysqld.sock
    host=localhost
`
		cfgCM := &corev1.ConfigMap{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), cfgCM)).Should(Succeed())
		Expect(checkedCreateObj(cfgCM)).Should(Succeed())
		return cfgCM
	}

	// config template对于了container的mountPath
	// configTemplateRefs:
	// 	 - name: mysql-tree-node-template-8.0
	//     volumeName: config1
	//   - name: mysql-tree-node2
	//     volumeName: config2
	// for containner
	// volumeMounts:
	//   #将my.cnf configmap mount到pod的指定目录下，/data/config
	//   #在pod中，会存在file: /data/config/my.cnf.override
	//   #polardb-x在entrypoint的脚本会将my.cnf.override合并到/data/mysql/conf/my.cnf文件中
	//   - mountPath: /data/config
	//     name: config1
	//   - mountPath: /etc/config
	//	   name: config2
	assureClusterDefObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefYAML := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    componentType: Stateful
    configTemplateRefs: 
    - name: mysql-tree-node-template-8.0 
      volumeName: mysql-config
    defaultReplicas: 1
    podSpec:
      containers:
      - name: mysql
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 3306
          protocol: TCP
          name: mysql
        - containerPort: 13306
          protocol: TCP
          name: paxos
        volumeMounts:
          - mountPath: /var/lib/mysql
            name: data
          - mountPath: /var/log
            name: log
          - mountPath: /data/config
            name: mysql-config
        env:
          - name: "MYSQL_ROOT_PASSWORD"
            valueFrom:
              secretKeyRef:
                name: $(OPENDBAAS_MY_SECRET_NAME)
                key: password
        command: ["/usr/bin/bash", "-c"]
        args:
          - >
            cluster_info="";
            for (( i=0; i<$OPENDBAAS_REPLICASETS_PRIMARY_N; i++ )); do
              if [ $i -ne 0 ]; then
                cluster_info="$cluster_info;";
              fi;
              host=$(eval echo \$OPENDBAAS_REPLICASETS_PRIMARY_"$i"_HOSTNAME)
              cluster_info="$cluster_info$host:13306";
            done;
            idx=0;
            while IFS='-' read -ra ADDR; do
              for i in "${ADDR[@]}"; do
                idx=$i;
              done;
            done <<< "$OPENDBAAS_MY_POD_NAME";
            echo $idx;
            cluster_info="$cluster_info@$(($idx+1))";
            echo $cluster_info;
            docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
  - typeName: proxy
    defaultReplicas: 1
    podSpec:
      containers:
      - name: nginx
    service:
      ports:
      - protocol: TCP
        port: 80
`
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		Expect(checkedCreateObj(clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	assureAppVersionObj := func() *dbaasv1alpha1.AppVersion {
		By("By assure an appVersion obj")
		appVerYAML := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind:       AppVersion
metadata:
  name:     app-version
spec:
  clusterDefinitionRef: cluster-definition
  components:
  - type: replicasets
    configTemplateRefs: 
    - name: mysql-tree-node-template-8.0 
      volumeName: mysql-config
    podSpec:
      containers:
      - name: mysql
        image: registry.jihulab.com/infracreate/mysql-server/mysql/wesql-server-arm:latest
  - type: proxy
    podSpec: 
      containers:
      - name: nginx
        image: nginx
`
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), appVersion)).Should(Succeed())
		Expect(checkedCreateObj(appVersion)).Should(Succeed())
		return appVersion
	}

	newClusterObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		appVersionObj *dbaasv1alpha1.AppVersion,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.AppVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and AppVersion objects if not provided
		if clusterDefObj == nil {
			assureCfgTplConfigMapObj("")
			clusterDefObj = assureClusterDefObj()
		}
		if appVersionObj == nil {
			appVersionObj = assureAppVersionObj()
		}

		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := types.NamespacedName{
			Name:      "cluster" + randomStr,
			Namespace: "default",
		}

		return &dbaasv1alpha1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "dbaas.infracreate.com/v1alpha1",
				Kind:       "Cluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterDefRef: clusterDefObj.GetName(),
				AppVersionRef: appVersionObj.GetName(),
			},
		}, clusterDefObj, appVersionObj, key
	}

	deleteClusterNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dbaasv1alpha1.Cluster{}
			if err := k8sClient.Get(context.Background(), key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(context.Background(), f)
		}()).Should(Succeed())

		var err error
		f := &dbaasv1alpha1.Cluster{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(context.Background(), key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(context.Background(), key, f) {
			f = &dbaasv1alpha1.Cluster{}
		}
		return client.IgnoreNotFound(err)
	}

	// Consensus associate objs
	// ClusterDefinition with componentType = Consensus
	assureClusterDefWithConsensusObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj with componentType = Consensus")
		clusterDefYAML := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-consensus
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    componentType: Consensus
    consensusSpec:
      leader:
        name: "leader"
        accessMode: ReadWrite
      followers:
      - name: "follower"
        accessMode: Readonly
      updateStrategy: Serial
    service:
      ports:
      - protocol: TCP
        port: 3306
    readonlyService:
      ports:
      - protocol: TCP
        port: 3306
    defaultReplicas: 3
    podSpec:
      containers:
      - name: mysql
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 3306
          protocol: TCP
          name: mysql
        - containerPort: 13306
          protocol: TCP
          name: paxos
        env:
        - name: MYSQL_ROOT_HOST
          value: '%'
        - name: MYSQL_ROOT_USER
          value: root
        - name: MYSQL_ROOT_PASSWORD
        - name: MYSQL_ALLOW_EMPTY_PASSWORD
          value: "yes"
        - name: MYSQL_DATABASE
          value: mydb
        - name: MYSQL_USER
          value: u1
        - name: MYSQL_PASSWORD
          value: u1
        - name: CLUSTER_ID
          value: "1"
        - name: CLUSTER_START_INDEX
          value: "1"
        - name: REPLICATIONUSER
          value: replicator
        - name: REPLICATION_PASSWORD
        - name: MYSQL_TEMPLATE_CONFIG
        - name: MYSQL_CUSTOM_CONFIG
        - name: MYSQL_DYNAMIC_CONFIG
        command: ["/bin/bash", "-c"]
        args:
          - >
            cluster_info="";
            for (( i=0; i<$OPENDBAAS_REPLICASETS_N; i++ )); do
              if [ $i -ne 0 ]; then
                cluster_info="$cluster_info;";
              fi;
              host=$(eval echo \$OPENDBAAS_REPLICASETS_"$i"_HOSTNAME)
              cluster_info="$cluster_info$host:13306";
            done;
            idx=0;
            while IFS='-' read -ra ADDR; do
              for i in "${ADDR[@]}"; do
                idx=$i;
              done;
            done <<< "$OPENDBAAS_MY_POD_NAME";
            echo $idx;
            cluster_info="$cluster_info@$(($idx+1))";
            echo $cluster_info;
            docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
  connectionCredential:
    user: root
`
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		Expect(checkedCreateObj(clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	assureAppVersionWithConsensusObj := func() *dbaasv1alpha1.AppVersion {
		By("By assure an appVersion obj with componentType = Consensus")
		appVerYAML := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind:       AppVersion
metadata:
  name:     app-version-consensus
spec:
  clusterDefinitionRef: cluster-definition-consensus
  components:
  - type: replicasets
    podSpec:
      containers:
      - name: mysql
        image: docker.io/infracreate/wesql-server-8.0:0.1-SNAPSHOT
        imagePullPolicy: IfNotPresent
`
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), appVersion)).Should(Succeed())
		Expect(checkedCreateObj(appVersion)).Should(Succeed())
		return appVersion
	}

	newClusterWithConsensusObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		appVersionObj *dbaasv1alpha1.AppVersion,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.AppVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and AppVersion objects if not provided
		if clusterDefObj == nil {
			assureCfgTplConfigMapObj("")
			clusterDefObj = assureClusterDefWithConsensusObj()
		}
		if appVersionObj == nil {
			appVersionObj = assureAppVersionWithConsensusObj()
		}

		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := types.NamespacedName{
			Name:      "cluster" + randomStr,
			Namespace: "default",
		}

		return &dbaasv1alpha1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "dbaas.infracreate.com/v1alpha1",
				Kind:       "Cluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterDefRef: clusterDefObj.GetName(),
				AppVersionRef: appVersionObj.GetName(),
				Components: []dbaasv1alpha1.ClusterComponent{
					{
						Name: "wesql-test",
						Type: "replicasets",
						VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{
							{
								Name: "data",
								Spec: corev1.PersistentVolumeClaimSpec{
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("1Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
		}, clusterDefObj, appVersionObj, key
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		By("AfterEach scope")
		Eventually(func() error {
			return deleteClusterNWait(clusterObjKey)
		}, timeout, interval).Should(Succeed())
	})

	Context("When creating cluster", func() {
		It("Should success with no error", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When deleting cluster", func() {
		It("Should delete cluster resources according to termination policy", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)

			toCreate.Spec.TerminationPolicy = dbaasv1alpha1.DoNotTerminate

			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			fetchedG1.Spec.TerminationPolicy = dbaasv1alpha1.Halt
			Expect(k8sClient.Update(context.Background(), fetchedG1)).Should(Succeed())

			fetchedG2 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG2)
				return fetchedG2.Status.ObservedGeneration == 2
			}, timeout, interval).Should(BeTrue())

			By("Deleting the cluster")
			Eventually(func() bool {
				if err := deleteClusterNWait(key); err != nil {
					return false
				}
				tmp := &dbaasv1alpha1.Cluster{}
				err := k8sClient.Get(context.Background(), key, tmp)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When updating cluster replicas", func() {
		It("Should create/delete pod to the replicas number", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(stsList.Items) != 0
			}, timeout, interval).Should(BeTrue())

			By("By updating replica")
			if fetchedG1.Spec.Components == nil {
				fetchedG1.Spec.Components = []dbaasv1alpha1.ClusterComponent{}
			}
			updatedReplicas := 5
			fetchedG1.Spec.Components = append(fetchedG1.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name:     "replicasets",
				Type:     "replicasets",
				Replicas: updatedReplicas,
			})
			Expect(k8sClient.Update(context.Background(), fetchedG1)).Should(Succeed())

			fetchedG2 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG2)
				return fetchedG2.Status.ObservedGeneration == 2
			}, timeout*2, interval).Should(BeTrue())

			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				Expect(len(stsList.Items) != 0).Should(BeTrue())
				return int(*stsList.Items[0].Spec.Replicas) == updatedReplicas
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should create deployment if component is stateless", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			deployList := &appsv1.DeploymentList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), deployList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(deployList.Items) != 0
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should create pdb if updateStrategy exists", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			pdbList := &policyv1.PodDisruptionBudgetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), pdbList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(pdbList.Items) != 0
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should create service if service configured", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Components = append(toCreate.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name: "proxy",
				Type: "proxy",

				ServiceType: "LoadBalancer",
			})
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			svcList := &corev1.ServiceList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), svcList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				for _, svc := range svcList.Items {
					if svc.Spec.Type == "LoadBalancer" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When updating cluster", func() {
		It("Should update PVC request storage size accordingly", func() {
			// this test required controller-manager component
			By("Check available controller-manager status")
			if !hasStorage(assureDefaultStorageClassObj) {
				return
			}

			By("By creating a cluster with volume claim")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Components = make([]dbaasv1alpha1.ClusterComponent, 1)
			toCreate.Spec.Components[0] = dbaasv1alpha1.ClusterComponent{
				Name: "replicasets1",
				Type: "replicasets",
				VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{
					{
						Name: "data",
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
					{
						Name: "log",
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				stsList := &appsv1.StatefulSetList{}
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())

				Expect(len(stsList.Items) == 1).Should(BeTrue())

				sts := &stsList.Items[0]
				Expect(sts.Spec.Replicas).ShouldNot(BeNil())
				return sts.Status.AvailableReplicas == *sts.Spec.Replicas
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				pvcList := &corev1.PersistentVolumeClaimList{}
				Expect(k8sClient.List(context.Background(), pvcList, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(pvcList.Items) != 0
			}, timeout*6, interval).Should(BeTrue())

			comp := &fetchedG1.Spec.Components[0]
			newStorageValue := resource.MustParse("2Gi")
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
			comp.VolumeClaimTemplates[1].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue

			Expect(k8sClient.Update(ctx, fetchedG1)).Should(Succeed())

			fetchedG2 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG2)
				return fetchedG2.Status.ObservedGeneration == 2
			}, timeout*2, interval).Should(BeTrue())

			// sts := &appsv1.StatefulSet{}
			// stsKey := types.NamespacedName{
			// 	Namespace: key.Namespace,
			// 	Name: fmt.Sprintf("%s-%s-%s",
			// 		key.Name,
			// 		fetchedG2.Spec.Components[0].Type,
			// 		fetchedG2.Spec.Components[0].Name),
			// }
			// Expect(k8sClient.Get(context.Background(), stsKey, sts)).Should(Succeed())

			stsList := &appsv1.StatefulSetList{}
			Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
				"app.kubernetes.io/instance": key.Name,
			}, client.InNamespace(key.Namespace))).Should(Succeed())

			for _, sts := range stsList.Items {
				for _, vct := range sts.Spec.VolumeClaimTemplates {
					for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
						pvc := &corev1.PersistentVolumeClaim{}
						pvcKey := types.NamespacedName{
							Namespace: key.Namespace,
							Name:      fmt.Sprintf("%s-%s-%d", vct.Name, sts.Name, i),
						}
						Expect(k8sClient.Get(context.Background(), pvcKey, pvc)).Should(Succeed())
						Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
					}
				}
			}

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout*2, interval).Should(Succeed())
		})
	})

	// Consensus associate test cases
	Context("When creating cluster with componentType = Consensus", func() {
		It("Should success with: "+
			"1 pod with 'leader' role label set, "+
			"2 pods with 'follower' role label set,"+
			"1 service routes to 'leader' pod and "+
			"1 service routes ro 'follower' pods", func() {
			By("By creating a cluster with componentType = Consensus")
			toCreate, _, _, key := newClusterWithConsensusObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			By("By waiting the cluster is created")
			cluster := &dbaasv1alpha1.Cluster{}

			// TODO: testEnv doesn't support pod creation yet. remove the following codes when it does
			if testEnv.UseExistingCluster == nil || !*testEnv.UseExistingCluster {
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), key, cluster)
					if err != nil {
						return false
					}

					return cluster.Status.Phase == dbaasv1alpha1.CreatingPhase
				}, timeout*3, interval*5).Should(BeTrue())

				return
			}
			// end remove

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), key, cluster)
				if err != nil {
					return false
				}

				return cluster.Status.Phase == dbaasv1alpha1.RunningPhase
			}, timeout*3, interval*5).Should(BeTrue())

			By("By checking pods' role label")
			observeRole := func(ip string, port int32) (string, error) {
				url := "root@tcp(" + ip + ":" + strconv.Itoa(int(port)) + ")/information_schema?allowNativePasswords=true"
				sql := "select role from information_schema.wesql_cluster_local"
				mysql := &Mysql{}
				params := map[string]string{connectionURLKey: url}
				err := mysql.Init(params)
				if err != nil {
					return "", err
				}

				result, err := mysql.query(context.Background(), sql)
				if err != nil {
					return "", err
				}
				if len(result) != 1 {
					return "", errors.New("only one role should be observed")
				}
				row, ok := result[0].(map[string]interface{})
				if !ok {
					return "", errors.New("query result wrong type")
				}
				role, ok := row["role"].(string)
				if !ok {
					return "", errors.New("role parsing error")
				}
				if len(role) == 0 {
					return "", errors.New("got empty role")
				}

				err = mysql.Close()
				role = strings.ToLower(role)
				if err != nil {
					return role, err
				}

				return role, nil
			}

			startPortForward := func(kind, name string, port int32) error {
				portStr := strconv.Itoa(int(port))
				cmd := exec.Command("bash", "-c", "kubectl port-forward "+kind+"/"+name+" --address 0.0.0.0 "+portStr+":"+portStr+" &")
				return cmd.Start()
			}

			stopPortForward := func(name string) error {
				cmd := exec.Command("bash", "-c", "ps aux | grep port-forward | grep -v grep | grep "+name+" | awk '{print $2}' | xargs kill -9")
				return cmd.Run()
			}

			ip := getLocalIP()
			Expect(ip).ShouldNot(BeEmpty())
			observeRoleOfPod := func(pod *corev1.Pod) string {
				kind := "pod"
				name := pod.Name
				port := pod.Spec.Containers[0].Ports[0].ContainerPort
				role := ""
				Eventually(func() bool {
					err := startPortForward(kind, name, port)
					if err != nil {
						_ = stopPortForward(name)
						return false
					}
					time.Sleep(interval)
					role, err = observeRole(ip, port)
					if err != nil {
						_ = stopPortForward(name)
						return false
					}
					_ = stopPortForward(name)

					return true
				}, timeout*2, interval*1).Should(BeTrue())

				return role
			}

			observeRoleOfService := func(svc *corev1.Service) string {
				kind := "svc"
				name := svc.Name
				port := svc.Spec.Ports[0].Port
				role := ""
				Eventually(func() bool {
					err := startPortForward(kind, name, port)
					if err != nil {
						_ = stopPortForward(name)
						return false
					}
					time.Sleep(interval)
					role, err = observeRole(ip, port)
					if err != nil {
						_ = stopPortForward(name)
						return false
					}
					_ = stopPortForward(name)

					return true
				}, timeout*2, interval*1).Should(BeTrue())

				return role
			}

			// TODO wait probe done @xuanchi
			// before that, we fake it. remove this
			updateRole := func(pod *corev1.Pod, role string) {
				patch := client.MergeFrom(pod.DeepCopy())
				pod.Labels[consensusSetRoleLabelKey] = role

				Expect(k8sClient.Patch(context.Background(), pod, patch)).Should(Succeed())
			}
			// end remove

			stsList := &appsv1.StatefulSetList{}
			Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
				"app.kubernetes.io/instance": key.Name,
			}, client.InNamespace(key.Namespace))).Should(Succeed())
			Expect(len(stsList.Items)).Should(Equal(1))
			sts := &stsList.Items[0]
			podList := &corev1.PodList{}
			Expect(k8sClient.List(context.Background(), podList, client.InNamespace(key.Namespace))).Should(Succeed())
			pods := make([]corev1.Pod, 0)
			for _, pod := range podList.Items {
				if isMemberOf(sts, &pod) {
					pods = append(pods, pod)
				}
			}

			// TODO set pod label, remove this after probe is done
			time.Sleep(interval * 2)
			for _, pod := range pods {
				role := observeRoleOfPod(&pod)
				updateRole(&pod, role)
			}
			// end remove

			// should have 3 pods
			Expect(len(pods)).Should(Equal(3))
			// 1 leader
			// 2 followers
			leaderCount, followerCount := 0, 0
			for _, pod := range pods {
				switch pod.Labels[consensusSetRoleLabelKey] {
				case leader:
					leaderCount++
				case follower:
					followerCount++
				}
			}
			Expect(leaderCount).Should(Equal(1))
			Expect(followerCount).Should(Equal(2))

			By("By checking services' status")
			// we should have 2 services
			svcList := &corev1.ServiceList{}
			Expect(k8sClient.List(context.Background(), svcList, client.MatchingLabels{
				"app.kubernetes.io/instance": key.Name,
			}, client.InNamespace(key.Namespace))).Should(Succeed())
			Expect(len(svcList.Items)).Should(Equal(2))
			svc, svcRo := svcList.Items[0], svcList.Items[1]
			roName := key.Name + "-" + cluster.Spec.Components[0].Name + "-ro"
			if svc.Name == roName {
				svc, svcRo = svcRo, svc
			}
			// getRole should be leader through service
			Expect(observeRoleOfService(&svc)).Should(Equal(leader))
			// getRole should be follower through readonlyService
			Expect(observeRoleOfService(&svcRo)).Should(Equal(follower))

			By("By deleting leader pod")
			leaderPod := &corev1.Pod{}
			followerPods := make([]*corev1.Pod, 0)
			for _, pod := range pods {
				switch pod.Labels[consensusSetRoleLabelKey] {
				case leader:
					leaderPod = &pod
				case follower:
					followerPods = append(followerPods, &pod)
				}
			}
			Expect(k8sClient.Delete(context.Background(), leaderPod)).Should(Succeed())
			time.Sleep(interval * 2)
			Eventually(func() bool {
				Expect(k8sClient.Get(context.Background(), types.NamespacedName{
					Namespace: sts.Namespace,
					Name:      sts.Name,
				}, sts)).Should(Succeed())
				return sts.Status.AvailableReplicas == 3
			}, timeout, interval).Should(BeTrue())
			// TODO role should be updated automatically after probe done
			// update it manually, remove this
			for _, pod := range followerPods {
				role := observeRoleOfPod(pod)
				updateRole(pod, role)
			}
			// end remove
			time.Sleep(interval * 2)
			Expect(observeRoleOfService(&svc)).Should(Equal(leader))

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

})

func hasStorage(assureDefaultStorageClassObj func() *storagev1.StorageClass) bool {
	csList := &corev1.ComponentStatusList{}
	_ = k8sClient.List(context.Background(), csList)
	isCMAvailable := false
	for _, cs := range csList.Items {
		if cs.Name != "controller-manager" {
			continue
		}
		for _, cond := range cs.Conditions {
			if cond.Type == "Healthy" && cond.Status == "True" {
				isCMAvailable = true
				break
			}
		}
	}
	if !isCMAvailable {
		// skip test if no available storage classes
		By("The controller-manager is not available, test skipped")
		return false
	}

	By("Check available storageclasses")
	scList := &storagev1.StorageClassList{}
	hasDefaultSC := false
	_ = k8sClient.List(context.Background(), scList)
	for _, sc := range scList.Items {
		annot := sc.Annotations
		if annot == nil {
			continue
		}
		if v, ok := annot["storageclass.kubernetes.io/is-default-class"]; ok && v == "true" {
			hasDefaultSC = true
			break
		}
	}
	if !hasDefaultSC {
		assureDefaultStorageClassObj()
	}

	return true
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

const (
	// configurations to connect to Mysql, either a data source name represent by URL.
	connectionURLKey = "url"

	// other general settings for DB connections.
	maxIdleConnsKey    = "maxIdleConns"
	maxOpenConnsKey    = "maxOpenConns"
	connMaxLifetimeKey = "connMaxLifetime"
	connMaxIdleTimeKey = "connMaxIdleTime"
)

// Mysql represents MySQL output bindings.
type Mysql struct {
	db *sql.DB
}

// Init initializes the MySQL binding.
func (m *Mysql) Init(metadata map[string]string) error {
	p := metadata
	url, ok := p[connectionURLKey]
	if !ok || url == "" {
		return fmt.Errorf("missing MySql connection string")
	}

	db, err := initDB(url)
	if err != nil {
		return err
	}

	err = propertyToInt(p, maxIdleConnsKey, db.SetMaxIdleConns)
	if err != nil {
		return err
	}

	err = propertyToInt(p, maxOpenConnsKey, db.SetMaxOpenConns)
	if err != nil {
		return err
	}

	err = propertyToDuration(p, connMaxIdleTimeKey, db.SetConnMaxIdleTime)
	if err != nil {
		return err
	}

	err = propertyToDuration(p, connMaxLifetimeKey, db.SetConnMaxLifetime)
	if err != nil {
		return err
	}

	err = db.Ping()
	if err != nil {
		return errors.Wrap(err, "unable to ping the DB")
	}

	m.db = db

	return nil
}

// Close will close the DB.
func (m *Mysql) Close() error {
	if m.db != nil {
		return m.db.Close()
	}

	return nil
}

func (m *Mysql) query(ctx context.Context, sql string) ([]interface{}, error) {
	rows, err := m.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", sql)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	result, err := m.jsonify(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshalling query result for %s", sql)
	}

	return result, nil
}

func propertyToInt(props map[string]string, key string, setter func(int)) error {
	if v, ok := props[key]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			setter(i)
		} else {
			return errors.Wrapf(err, "error converitng %s:%s to int", key, v)
		}
	}

	return nil
}

func propertyToDuration(props map[string]string, key string, setter func(time.Duration)) error {
	if v, ok := props[key]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			setter(d)
		} else {
			return errors.Wrapf(err, "error converitng %s:%s to time duration", key, v)
		}
	}

	return nil
}

func initDB(url string) (*sql.DB, error) {
	if _, err := mysql.ParseDSN(url); err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", connectionURLKey)
	}

	db, err := sql.Open("mysql", url)
	if err != nil {
		return nil, errors.Wrap(err, "error opening DB connection")
	}

	return db, nil
}

func (m *Mysql) jsonify(rows *sql.Rows) ([]interface{}, error) {
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	var ret []interface{}
	for rows.Next() {
		values := prepareValues(columnTypes)
		err := rows.Scan(values...)
		if err != nil {
			return nil, err
		}

		r := m.convert(columnTypes, values)
		ret = append(ret, r)
	}

	return ret, nil
}

func prepareValues(columnTypes []*sql.ColumnType) []interface{} {
	types := make([]reflect.Type, len(columnTypes))
	for i, tp := range columnTypes {
		types[i] = tp.ScanType()
	}

	values := make([]interface{}, len(columnTypes))
	for i := range values {
		values[i] = reflect.New(types[i]).Interface()
	}

	return values
}

func (m *Mysql) convert(columnTypes []*sql.ColumnType, values []interface{}) map[string]interface{} {
	r := map[string]interface{}{}

	for i, ct := range columnTypes {
		value := values[i]

		switch v := values[i].(type) {
		case driver.Valuer:
			if vv, err := v.Value(); err == nil {
				value = interface{}(vv)
			}
		case *sql.RawBytes:
			// special case for sql.RawBytes, see https://github.com/go-sql-driver/mysql/blob/master/fields.go#L178
			switch ct.DatabaseTypeName() {
			case "VARCHAR", "CHAR", "TEXT", "LONGTEXT":
				value = string(*v)
			}
		}

		if value != nil {
			r[ct.Name()] = value
		}
	}

	return r
}
