 # GCS Storage Management

 ## Table of contents
 - [Artifact Tagging in GCS](#tagging)
 - [Cleanup in GCP](#cleanup)

 ## Artifact Tagging in GCS<a name="tagging"></a>

In GCP, [custom metadata](https://cloud.google.com/storage/docs/metadata#custom-metadata) can be applied to stored artifacts as a means of labelling those objects.
By employing a sensible tagging strategy, objects can be easily selected and managed using custom metadata.

Custom metadata takes the form of `key=value` pairs, both of which are entirely customisable (although non-ascii characters should be avoided). Any number of `key=value` pairs can be applied to an object. Custom metadata associated with an object can be updated and/or deleted at any time.

It should be noted that custom metadata in GCP is subject to a [size limit](https://cloud.google.com/storage/quotas#objects) and incurs [storage costs](https://cloud.google.com/storage/pricing#storage-notes).

GCP Storage uses the concept of virtual folders where folders themselves don't actually exist and are simulated as a result of hierarchical naming of objects (e.g. *Android/Builds/AAOSBuilder/01/build_info.txt*); as a result, custom metadata can only be applied to objects, not folders. Instead of tagging an entire folder, all objects in the folder can be tagged.

### Setting / Manipulation of Custom Metadata On New Objects:
The following build jobs provide a parameter ``STORAGE LABELS`` which allows users to add labels to the artifacts being uploaded to storage buckets. When using the GCP storage option, these labels are implemented as custom metadata.

- Android / AAOS Builder
- Android / AAOS Builder ABFS
- OpenBSW / BSW Builder

### Setting / Manipulation of Custom Metadata On Existing Objects:
The following utility jobs can be used to view, add, modify and delete custom metadata on individual objects or groups of objects (grouped by folder or by existing metadata via filtering)

- **Object - List Metadata**
This job allows the user to inspect the metadata of objects stored in a GCS bucket.

- **Object - Add Metadata**
This job allows the user to add metadata (key/value pairs) to objects stored in a GCS bucket.

- **Object - Remove Metadata**
This job allows the user to remove specify or all metadata from objects stored in a GCS bucket.

- **Filter Objects by Metadata**
This job allows the user to list all objects in a bucket path based on the metadata that is set on them.
The user can choose to list objects with specific metadata, objects with any metadata or objects with no metadata.

- **Filtered Objects - Remove Metadata**
This job allows the user to find all objects in the bucket which have a specified metadata item set and remove that metadata item from the objects.

- **Filtered Objects - Update Metadata**
This job allows the user to find all objects in a bucket path which have the specified metadata set and update the value of that custom metadata item.


## Cleanup in GCP<a name="cleanup"></a>

### Using Lifecycle Policies<a name="lifecycle"></a>
In GCP, [lifecycle](https://cloud.google.com/storage/docs/lifecycle) management is done at a bucket level.

Lifecycle configurations contain a set of rules, each of which contains an [action](https://cloud.google.com/storage/docs/lifecycle#actions) (e.g. delete) and [conditions](https://cloud.google.com/storage/docs/lifecycle#conditions) (e.g. age=100days). When any object in the bucket meets all conditions, the specified action is taken.

In GCP ,lifecycle conditions can be based on the following:

- object name prefix or suffix
- fixed-key metadata set on the object

A limitation with GCP is that custom metadata cannot be used to create lifecycle management rules. However, a workaround is explained in a <a name="cleanup_metadata">following section</a>.

### Using Storage Classes<a name="storage_class"></a>
The [storage class](https://cloud.google.com/storage/docs/storage-classes) of a GCP object determines its availability and storage cost.

The default storage class used for a bucket is set during the bucket setup process and all uploaded objects inherit this default storage class. The storage class of any object can be changed explicitly.

Storage classes can be used in lifecycle management policies in order to manage costs of objects which need to be retained for extended periods.

### Using Custom Metadata<a name="cleanup_metadata"></a>
The following utility jobs have been provided so that the user can perform cleanup operations on GCP objects based on the custom metadata that is set on them.

- **Object - List Storage Class**
This job allows the user to inspect the storage class of objects stored in a GCS bucket.

- **Filtered Objects - Delete**
This job allows the user to find all objects in a bucket path which have the specified metadata set and delete those objects.

- **Filtered Objects - Move**
This job allows the user to find all objects in a bucket path which have the specified metadata set and change the storage class of those objects.

