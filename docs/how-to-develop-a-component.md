# How to develop a Component?

**This file is under construction**

## Decide about the general deployment?

Decide which type of landscaper component fits best to your demands? Alternatives are [helm](https://github.com/gardener/landscaper/blob/master/docs/deployer/helm.md), [container](https://github.com/gardener/landscaper/blob/master/docs/deployer/container.md) or [manifest](https://github.com/gardener/landscaper/blob/master/docs/deployer/manifest.md) (more will follow). 

- Helm: Select a helm component if you want to deploy some helm chart ([example](https://github.com/gardener/landscaper/blob/master/docs/tutorials/01-create-simple-blueprint.md)).

- Container: If you need some computations during your deployment, e.g. the generation of certificates, go for a container component ([example](https://github.com/achimweigel/virtual-garden/tree/landscaper-component)).

- Manifest: I you just need to deploy some kubernetes manifests create a manifest component. ([example](https://github.com/achimweigel/gardener-extension-networking-calico/tree/landscaper-local-dev-test2))

## Create the blueprint

Create a blueprint in the git repository of your project in a folder *`./landscaper/blueprint`* mainly consisting of ([simple blueprint example](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/.landscaper/blueprint),  [blueprint example with export data](https://github.com/achimweigel/virtual-garden/tree/landscaper-component/.landscaper/blueprint))

- Import section
- Deploy items
- Export section

More details could be found in the [tutorials](https://github.com/gardener/landscaper/tree/master/docs/tutorials).

**Important**: All images in the component must be referenced via the component descriptor. Otherwise they could not be transported, scanned etc. ([example](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/.landscaper/blueprint/deploy-executions.yaml#L26-L28))

**Remark:** You could verify your blueprint locally using the render/validate command of the [landscaper-cli](https://github.com/gardener/landscapercli/blob/master/docs/installation.md) `landscaper-cli blueprints render` or `landscaper-cli blueprints validate`. 

## Define the resources added to the component descriptor

Next you have to specify additional resources which should be added to the component descriptor. These are mainly the blueprint and images which are not already specified .ci/pipeline_definitions and therefore automatically added. The additional resources should be specified in a file *`.landscaper/resources.yaml`*.

Example:

- [virtual garden](https://github.com/gardener/virtual-garden/blob/master/.landscaper/resources.yaml): containing the blueprint, the external images for the api server and the etcd.

- [calico extension](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/.landscaper/resources.yaml): containing the blueprint and the helm chart which is included in the manifests. 

## Build and upload the component descriptor for your component for dev testing

**Initial Remark**: If you have authentication problems when uploading resources to a OCI registry you need to login either via `gcloud auth login` or `gcloud auth activate-service-account --key-file=<path to your keyfile>`

- Provide basic script for creating a component descriptor used by the pipeline ([see](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/.ci/component_descriptor)). Adapt the naming in the script if needed. 

- Though this step is only needed by the pipeline, it is recommended to add the component_descriptor traits to the [pipeline definitions](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/.ci/pipeline_definitions). 

Add some makefile targets to build the resources you need for the last commit hash:

- Build images (see target [docker-images](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/Makefile#L54))

- Push images (see target [cnudie-docker-push](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/Makefile#L121))

- Build and push component descriptor (see target [cnudie-cd-build-push](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/Makefile#L127))

  - You need to add and adapt a script [environment.sh](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/hack/environment.sh)
    - insert `eu.gcr.io/sap-se-gcr-k8s-private/cnudie/gardener/development` for the registry in the function `get_cd_registry`
    - adapt `get_cd_component_name` and `get_image_registry` such that it fits to your component.

   - You need to add and adapt a script [generate-cd.sh](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/hack/generate-cd.sh)

      - Mainly add all resources to the component descriptor only referenced in the pipeline definitions but not the resources.yaml ([see](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/hack/generate-cd.sh#L45-L54))

The component descriptor is uploaded to eu.gcr.io/`sap-se-gcr-k8s-private/cnudie/gardener/development/component-descriptors/...`

## Create an installation

- Add and adapt a script to create an installation for your component ([see](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/hack/create-installation.sh))

  - adapt the [name](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/hack/create-installation.sh#L22) of the installation

  - adapt the [referenced blueprint](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/hack/create-installation.sh#L34) of the installation

  - provide some [fix input values](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/hack/create-installation.sh#L34) for your component. Then you do not need to create some input data during testing.

- add a [make file target](https://github.com/achimweigel/gardener-extension-networking-calico/blob/landscaper-local-dev-test2/Makefile#L131) to create the installation

## Deploy installation on central dev landscaper

You could also install your own landscaper ([docu](https://github.com/gardener/landscaper/blob/master/docs/gettingstarted/install-landscaper-controller.md)). This requires also configuring the access credentials to the OCI registry.

For testing purposes it is easier to use the central landscaper of the dev landscape. The kubeconfig for its shoot cluster could be found [here](https://github.wdf.sap.corp/kubernetes-dev/cluster-dev-landscaper-gke/blob/master/gen/assets/auth/kubeconfig)

- Create a new namespace for your test installation on the landscaper cluster

- Install [landscaper-cli](https://github.com/gardener/landscapercli/blob/master/docs/installation.md)

- For your installation you need to create a target object in your new namespace as input data for your installation with the landscaper-cli ([link](https://github.com/gardener/landscapercli/blob/master/docs/commands/targets/create.md))

- Deploy the installation in the new namespace in the landscaper cluster and check its status.

## Transport component to landscapes

To be done when stuff is merged.

## Deploy script in landscape setup (including transport of component descriptor)

To be done.


