package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	privatev1alpha1 "github.com/thetechnick/orlop-gcp-hcp/api/private/v1alpha1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(privatev1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "mhc-scheduler.orlop-gcp-hcp.thetechnick.github.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&ManagedHostedClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ManagedHostedCluster")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

type ManagedHostedClusterReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	clusterIdx int
	clusters   []string
}

func (r *ManagedHostedClusterReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	var mhc privatev1alpha1.ManagedHostedCluster
	if err := r.Get(ctx, req.NamespacedName, &mhc); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	scheduledType := "private.orlop.thetechnick.ninja/Scheduled"
	if mhc.Spec.ManagementClusterName != "" {

		if meta.IsStatusConditionPresentAndEqual(mhc.Status.Conditions, scheduledType, metav1.ConditionTrue) &&
			meta.IsStatusConditionPresentAndEqual(mhc.Status.Conditions, "Available", metav1.ConditionTrue) {
			return reconcile.Result{}, nil
		}

		scheduledCondition := metav1.Condition{
			Type:               scheduledType,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: mhc.Generation,
			LastTransitionTime: metav1.Now(),
			Reason:             "Scheduled",
			Message:            fmt.Sprintf("Scheduled to management cluster: %s", mhc.Spec.ManagementClusterName),
		}
		meta.SetStatusCondition(&mhc.Status.Conditions, scheduledCondition)
		meta.SetStatusCondition(&mhc.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: mhc.Generation,
			LastTransitionTime: metav1.Now(),
			Reason:             "Available",
			Message:            "Your HostedCluster is up and running.",
		})

		if err := r.Status().Update(ctx, &mhc); err != nil {
			log.Error(err, "failed to update status")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	cluster := r.scheduleCluster()
	log.Info("scheduling ManagedHostedCluster", "cluster", cluster)

	mhc.Spec.ManagementClusterName = cluster

	if err := r.Update(ctx, &mhc); err != nil {
		log.Error(err, "failed to update ManagedHostedCluster")
		return reconcile.Result{}, err
	}

	scheduledCondition := metav1.Condition{
		Type:               scheduledType,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: mhc.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             "Scheduled",
		Message:            fmt.Sprintf("Scheduled to management cluster: %s", cluster),
	}
	meta.SetStatusCondition(&mhc.Status.Conditions, scheduledCondition)

	if err := r.Status().Update(ctx, &mhc); err != nil {
		log.Error(err, "failed to update status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ManagedHostedClusterReconciler) scheduleCluster() string {
	if len(r.clusters) == 0 {
		r.clusters = []string{"cluster-1", "cluster-2"}
	}

	cluster := r.clusters[r.clusterIdx]
	r.clusterIdx = (r.clusterIdx + 1) % len(r.clusters)
	return cluster
}

func (r *ManagedHostedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&privatev1alpha1.ManagedHostedCluster{}).
		Complete(r)
}
