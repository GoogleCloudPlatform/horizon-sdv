#!/usr/bin/env bash
# Copyright (c) 2025 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

BUCKET_PATH=${BUCKET_PATH:-}
URL_PATH=${URL_PATH:-}
FILTER_TYPE=${FILTER_TYPE:-}
KEYVALUE_PAIRS=${KEYVALUE_PAIRS:-}
KEYS=${KEYS:-}
KEYVALUE_PAIR=${KEYVALUE_PAIR:-}
UPDATE_VALUE=${UPDATE_VALUE:-}
STORAGE_CLASS=${STORAGE_CLASS:-}
FILE=${FILE:-}

RESULT=1
LIST_METADATA_WITH_OBJECT="false"



# Creates the metadata string used for adding / updating gcs object info
# Returns output on stdout (empty string if input is invalid/empty)
# Parameters:   <key_value_pairs>   key=value pairs as a single string (separated by , and/or space)
# Input:        create_metadata_string "key1=1 key2=2,key3=300 key4=true"
# Output:       "--custom-metadata=key1=1,key2=2,key3=300,key4=true"
function create_metadata_string(){
    # input must be a single string
    input_string=$1
    cleaned_string=$(echo "$input_string" | tr -s ' '| tr ' ' ','| tr -s ',')

    validated_metadata=""
    # ensure that all metadata items are valid
    IFS=',' read -ra keyvalues <<< "$cleaned_string"
    for keyvalue in "${keyvalues[@]}"; do
        # ensure the keyvalue is valid: '<key>=<value>'
        if [[ "$keyvalue" =~ ^[^=]+=[^=]+$ ]]; then
            validated_metadata+="${keyvalue},"
        fi
    done
    validated_metadata="${validated_metadata%,}"  # Remove trailing comma

    if [[ "$validated_metadata" != "" ]]; then
        echo "--custom-metadata=$validated_metadata"
    else
        echo ""
    fi
}

# Outputs the custom metadata associated with an object, excluding the key/value pairs
# that match the provided (space-separated) keys and
# Output is formatted such that it can be used to apply to another object
# Returns output on stdout
# Parameters:   <object_path> <space_separated_keys>
# Input         get_metadata_string_with_exclusions "gs://bucketname/foldername/objectname" "keyA keyB"
# Output:       keyC=3,keyD=4
function get_metadata_string_with_exclusions(){
    local ob_path="$1"
    shift
    local keys_to_remove_str="$1"
    local initial_metadata_string=""
    local final_metadata_string=""
    local keys_to_remove
    IFS=' ' read -ra keys_to_remove <<< "$keys_to_remove_str"

    if ! raw_metadata=$(gcloud storage objects describe "$ob_path" --format="text(custom_fields)"); then
        echo "GCS Error - Failed to get object metadata for $ob_path"
        echo "$raw_metadata"
        return 1
    fi
    if [[ $raw_metadata == ": None" ]]; then
        echo ""
        return
    fi

    cleaned_metadata=$(echo "$raw_metadata" | tr -d ' ')           # remove all spaces
    cleaned_metadata=$(echo "$cleaned_metadata" | tr ':' '=')      # Replace ': '' with '=''

    # example line: custom_fields.key=value
    local line
    while read -r line; do
        line="${line#custom_fields.}"       # remove the custom_fields. prefix

        key=$(echo "${line%%=*}" | xargs)   # remove everything from = to end of string (i.e. remove the :value bit)
        value=$(echo "${line#*=}" | xargs)  # remove everything from the start of the string to the :

        skip_key=false
        local remove_key
        for remove_key in "${keys_to_remove[@]}"; do
            if [[ "$key" == "$remove_key" ]]; then
                skip_key=true
                break
            fi
        done

        initial_metadata_string+="${key}=${value},"
        if ! $skip_key; then
            final_metadata_string+="${key}=${value},"
        fi

    done <<< "$cleaned_metadata"

    while read -r line; do
        line="${line#custom_fields.}"       # remove the custom_fields. prefix

        key=$(echo "${line%%=*}" | xargs)   # remove everything from = to end of string (i.e. remove the :value bit)
        value=$(echo "${line#*=}" | xargs)  # remove everything from the start of the string to the :

    done <<< "$cleaned_metadata"

    initial_metadata_string="${initial_metadata_string%,}"  # Remove trailing comma
    final_metadata_string="${final_metadata_string%,}"  # Remove trailing comma

    echo "$final_metadata_string"
}

# Outputs all objects in a bucket path which
# either have custom metadata set or not set
# Returns output on stdout
# Parameters:   <bucket_path>                           path must end in / or /*
#               <expected_custom_metadata_is_present>   true or false
# Input:        list_objects_custom_metadata_status "gs://bucketname/foldername/*" true
# Output:       gs://bucketname/foldername/object1name
#               gs://bucketname/foldername/object2name
#               gs://bucketname/foldername/subfolder/object3name
function list_objects_custom_metadata_status(){
    local bucket_path="$1"
    local expected_custom_metadata_is_present="$2"
    local actual_custom_metadata_is_present

    # create object list
    local object_list
    object_list=$(get_object_list "$bucket_path")

    while IFS= read -r object_path; do
        if ! metadata=$(gcloud storage objects describe "$object_path" --format=json 2>/dev/null) ; then
            echo "GCS Error - Failed to get object metadata for $object_path"
            echo "$metadata"
            return 1
        fi

        custom_metadata=$(echo "$metadata" | jq '.custom_fields')

        actual_custom_metadata_is_present=false
        if [[ "$custom_metadata" != "null" ]]; then
            actual_custom_metadata_is_present=true
        fi

        if [[ "$expected_custom_metadata_is_present" == "$actual_custom_metadata_is_present" ]]; then
            echo "$object_path"
        fi

    done <<< "$object_list"
}

# Outputs the metadata on all objects in the provided list
# Returns output on stdout
# Parameters:   <object_list>
# Input:        objectlist_list_metadata "$object_list"
function objectlist_list_metadata(){
    local object
    while IFS= read -r object; do
        if ! list_metadata "$object" false ; then
            return 1
        fi
    done <<< "$1"
}

# Outputs a list of objects for path input
# Returns output on stdout
# Parameters:   <path to object or folder>
# Input:        get_object_list "gs://bucketname/foldername/objectname"
# Output:       "gs://bucketname/foldername/objectname"
# Input:        get_object_list "gs://bucketname/foldername/"
# Output:       list of objects in the folder
function get_object_list(){
    local url_path="$1"

    # test if input is a folder or an object
    if [[ "$url_path" == */ || "$url_path" == */\* ]]; then
        url_path="${url_path%%\**}"   # remove all trailing *
        gcloud storage ls "$url_path**"
    else
        echo "$url_path"
    fi
}

# Uploads a file or multiple files to GCS bucket and optionally includes metadata
# Parameters: <file_path> <gcs_bucket_path> <key1>=<value1> <key2>=<value2>
# Input:    object_upload /aaos-cache/aaos_builds/build_info.txt gs://bucketname/foldername/ key1=1 key2=2
function object_upload(){
    file=$1
    bucket_path=$2
    shift
    shift
    metadata=$*
    local metadata_string
    local exit_code
    local copycmd
    [ -d "${file}" ] && copycmd="cp -r" || copycmd="cp"

    if [[ "$metadata" != "" ]]; then
        metadata_string=$(create_metadata_string "$metadata")
        echo "gcloud storage ${copycmd} ${file} ${bucket_path}/ $metadata_string"
        gcloud storage "${copycmd}" "${file}" "${bucket_path}"/ "$metadata_string"
        exit_code=$?
        echo "Copied ${file} to ${bucket_path} $metadata_string"
    else
        echo "gcloud storage ${copycmd} ${file} ${bucket_path}/"
        gcloud storage "${copycmd}" "${file}" "${bucket_path}"/
        exit_code=$?
        echo "Copied ${file} to ${bucket_path}"
    fi
    return $exit_code
}

# Changes the storage class of all objects in the provided list
# Returns output on stdout
# Parameters:   <object_list> <storage_class>
# Input:        objectlist_move_storage "$object_list"
function objectlist_move_storage(){
    local object_list="$1"
    local storage_class="$2"
    local object_path
    while IFS= read -r object_path; do
        if ! gcloud storage objects update "$object_path" --storage-class="$storage_class" ; then
            echo "GCS Error - Failed to move object(s) at $object_path to $storage_class storage"
            return 1
        fi
    done <<< "$object_list"
}

# Delete all objects in the provided list
# Returns output on stdout
# Parameters:   <object_list>
# Input:        objectlist_delete_all "$object_list"
function objectlist_delete_all(){
    local object_path
    while IFS= read -r object_path; do
        if ! gcloud storage rm "$object_path" ; then
            echo "GCS Error - Failed to delete object(s) at $object_path"
            return 1
        fi
    done <<< "$1"
}

# Outputs the storage class of
# - an object
# - all objects in the specified parent path (specified with a trailing / or /*)
# Returns output on stdout
# Parameters:   <object_or_parent_path>
# Input:        list_storage_class gs://bucketname/foldername/objectname
# Output:       gs://bucketname/foldername/objectname STANDARD
function list_storage_class(){
    local url_path="$1"

    # create object list
    local object_list
    object_list=$(get_object_list "$url_path")

    # perform metadata query
    local object_path
    while IFS= read -r object_path; do
        if ! metadata=$(gcloud storage objects describe "$object_path" --format=json) ; then
            echo "GCS Error - Failed to get object metadata for $object_path."
            echo "$metadata"
            return 1
        fi
        storage_class=$(echo "$metadata" | jq -r '.storage_class')
        echo "$object_path $storage_class"
    done <<< "$object_list"
}

# Adds or Updates any number of custom metadata key/value pairs to either
# - an object
# - all objects in the specified parent path (specified with a trailing / or /*)
# Parameters:   <object_or_parent_path> <key1>=<value1> <key2>=<value2> ....
# Input:        add_metadata "gs://bucketname/foldername/objectname" "key1=1 key2=2"
function add_metadata(){
    local url_path="$1"
    shift
    local metadata_as_str=$*

    # create object list
    local object_list
    object_list=$(get_object_list "$url_path")

    # perform custom metadata update
    local metadata_string
    local object_path
    metadata_string=$(create_metadata_string "$metadata_as_str")
    if [[ "$metadata_string" == "" ]]; then
        echo "Provided metadata is not valid: <$metadata_as_str>"
        return 1
    fi
    while IFS= read -r object_path; do
        if ! gcloud storage objects update "$object_path" "$metadata_string" ; then
            echo "GCS Error - Failed to add object metadata to $object_path. Metadata: $metadata_string"
            return 1
        fi
    done <<< "$object_list"
}

# Outputs all custom metadata associated with either
# - an object
# - all objects in the specified parent path (specified with a trailing / or /*)
# Returns output on stdout
# Parameters:   <object_or_parent_path>
# Input:        list_metadata gs://bucketname/foldername/objectname
# Output:       gs://bucketname/foldername/objectname keyA=1;keyB=2;keyC=3
function list_metadata(){
    local url_path="$1"

    # create object list
    local object_list
    object_list=$(get_object_list "$url_path")

    # perform metadata query
    local object_path
    while IFS= read -r object_path; do
        if ! metadata=$(gcloud storage objects describe "$object_path" --format="value(custom_fields)") ; then
            echo "GCS Error - Failed to get object metadata for $object_path."
            echo "$metadata"
            return 1
        fi
        echo "$object_path $metadata"
    done <<< "$object_list"
}

# Removes all custom metadata from either
# - an object
# - all objects in the specified parent path (specified with a trailing / or /*)
# Parameters:   <object_or_parent_path>
# Input:        remove_all_metadata gs://bucketname/foldername/objectname
function remove_all_metadata(){
    local url_path="$1"

    # create object list
    local object_list
    object_list=$(get_object_list "$url_path")

    # perform custom metadata removal
    local object_path
    while IFS= read -r object_path; do
        if ! gcloud storage objects update "$object_path" --clear-custom-metadata ; then
            echo "GCS Error - Failed to remove object metadata from $object_path"
            return 1
        fi
    done <<< "$object_list"
}

# Removes the specified custom metadata from either
# - an object
# - all objects in the specified parent path (specified with a trailing / or /*)
# Parameters:   <object_or_parent_path> <key1> <key2> ....
# Input:        remove_metadata gs://bucketname/foldername/objectname keyA keyB
function remove_metadata(){
    local url_path="$1"
    shift
    local keys_to_remove=("$@")
    local keys_as_string="${keys_to_remove[*]}"

    # create object list
    local object_list
    object_list=$(list_objects_filtered_metadata "$url_path" "$keys_as_string")

    # perform custom metadata update

    local object_path
    while IFS= read -r object_path; do

        updated_metadata=$(get_metadata_string_with_exclusions "$object_path" "$keys_as_string")

        if [ "$updated_metadata" == "1" ]; then
            return 1
        fi

        remove_all_metadata "$object_path"
        if [ $? -eq 1 ]; then
            return 1
        fi

        if [[ "$updated_metadata" != "" ]]; then
            add_metadata "$object_path" "$updated_metadata"
            if [ $? -eq 1 ]; then
                return 1
            fi
        fi

    done <<< "$object_list"
}

# Outputs all objects in a bucket path which have custom metadata
# set which matches the provided key(s) and value (if provided)
# Returns output on stdout
# Parameters:   <bucket_path>       path must end in / or /*
#               <key_or_key=value> <key_or_key=value> <key_or_key=value>
# Input:        list_objects_filtered_metadata "gs://bucketname/foldername/*" "key1=1 key2"
# Output:       gs://bucketname/foldername/object1name
#               gs://bucketname/foldername/object2name
#               gs://bucketname/foldername/subfolder/object3name
function list_objects_filtered_metadata(){
    local bucket_path="$1"
    shift

    local expected_keyvalues_as_str=$*
    expected_keyvalues_as_str=$(echo "$expected_keyvalues_as_str" | tr -s ' '| tr ' ' ','| tr -s ',')

    # create object list
    local object_list
    object_list=$(get_object_list "$bucket_path")

    while IFS= read -r object_path; do
        if ! metadata=$(gcloud storage objects describe "$object_path" --format=json 2>/dev/null) ; then
            echo "GCS Error - Failed to get object metadata for $object_path"
            echo "$metadata"
            return 1
        fi

        object_match="true"
        key_string=""

        IFS=',' read -ra expected_keyvalues <<< "$expected_keyvalues_as_str"
        for expected_keyvalue in "${expected_keyvalues[@]}"; do

            if [[ "$object_match" == "false" ]]; then
                # previous keys do not meet expectation - abandon search"
                break
            fi

            # define key and value
            if [[ "$expected_keyvalue" == *"="* ]]; then
                expected_key="${expected_keyvalue%%=*}"
                expected_value="${expected_keyvalue#*=}"
            else
                expected_key="$expected_keyvalue"
                expected_value=""
            fi

            # extract actual value assigned to key from metadata
            actual_value=$(echo "$metadata" | jq -r ".custom_fields[\"$expected_key\"]")

            # continue if metadata doesn't contains the key in question (i.e. if it has no value for the key)
            if [[ -z "$actual_value" || "$actual_value" == "null" ]]; then
                object_match="false"
                continue
            fi

            # metadata does contain the key in question

            # continue if expected value is provided but doesn't match the actual value
            if [[ "$expected_value" != "" && "$expected_value" != "$actual_value" ]]; then
                object_match="false"
                continue
            fi

            key_string+="$expected_key=$actual_value "

        done

        if [[ "$object_match" == "true" ]]; then
            if [[ "$key_string" == "" || "$LIST_METADATA_WITH_OBJECT" == "false" ]]; then
                echo "$object_path"
            else
                echo "$object_path $key_string"
            fi
            continue
        fi

    done <<< "$object_list"

}

# Finds all objects in a bucket path which have a custom metadata key or key/value pair
# which matches the provided one and updates the value assigned to that key.
# Outputs the before and after states to stdout
# Parameters:   <bucket_path> <original_key_or_key=value> <new_value>
# Input:        update_metadata_filtered_objects "gs://bucketname/foldername/*" "keyA=1" "100"
# Output:               gs://bucketname/foldername/object1name keyA=1;keyB=2
#                       gs://bucketname/foldername/object2name keyA=10;keyC=3
#               Result: gs://bucketname/foldername/object1name keyA=100;keyB=2
#                       gs://bucketname/foldername/object2name keyA=100;keyC=3
function update_metadata_filtered_objects(){
    local bucket_path="$1"
    local orig_keyvalue="$2"
    local orig_key="${orig_keyvalue%%=*}"
    local new_value="$3"
    local object_list
    local object_path

    # get the list of objects who already have the specified metadata set
    object_list=$(list_objects_filtered_metadata "$bucket_path" "$orig_keyvalue")

    if [[ -z "${object_list}" ]]; then
        echo "GCS Error: no objects found in $bucket_path with $orig_keyvalue"
        return 1
    fi

    # output the "before" state
    objectlist_list_metadata "$object_list"
    if [ $? -eq 1 ]; then
        return 1
    fi

    # perform custom metadata update - add the new metadata (add will over-write existing value)
    while IFS= read -r object_path; do
        add_metadata "$object_path" "$orig_key=$new_value"
        if [ $? -eq 1 ]; then
            return 1
        fi
    done <<< "$object_list"

    # output the "after" state
    echo "Result:"
    objectlist_list_metadata "$object_list"
    if [ $? -eq 1 ]; then
        return 1
    fi
}

# Finds all objects in a bucket path which have a custom metadata key or key/value pair
# which matches the provided one and deletes that metadata item.
# Outputs the before and after states to stdout
# Parameters: <bucket_path> <original_key_or_key=value>
# Input:        remove_metadata_filtered_objects "gs://bucketname/foldername/*" "keyA=1"
# Output:               gs://bucketname/foldername/object1name keyA=1;keyB=2
#               Result: gs://bucketname/foldername/object1name keyB=2
function remove_metadata_filtered_objects(){
    local bucket_path="$1"
    local orig_keyvalue="$2"
    local orig_key="${orig_keyvalue%%=*}"
    local object_list
    local object_path

    # get the list of objects who already have the specified metadata set
    object_list=$(list_objects_filtered_metadata "$bucket_path" "$orig_keyvalue")

    if [[ -z "${object_list}" ]]; then
        echo "no objects found in $bucket_path with $orig_keyvalue"
        return 1
    fi

    # output the "before" state
    objectlist_list_metadata "$object_list"
    if [ $? -eq 1 ]; then
        return 1
    fi

    # perform custom metadata update
    while IFS= read -r object_path; do
        remove_metadata "$object_path" "$orig_key"
        if [ $? -eq 1 ]; then
            return 1
        fi
    done <<< "$object_list"

    # output the "after" state
    echo "Result:"
    objectlist_list_metadata "$object_list"
    if [ $? -eq 1 ]; then
        return 1
    fi
}

# Main
case "${1}" in
    LIST_OBJECT_METADATA)
        printf '%s\n' \
        "---------------------------------------" \
        "List Object Metadata for $URL_PATH" \
        "---------------------------------------" 2>&1 | tee -a output.txt
        list_metadata "$URL_PATH" 2>&1 | tee -a output.txt
        RESULT=${PIPESTATUS[0]}
        ;;
    LIST_OBJECT_STORAGE_CLASS)
        printf '%s\n' \
        "---------------------------------------" \
        "List Object Storage Class for $URL_PATH" \
        "---------------------------------------" 2>&1 | tee -a output.txt
        list_storage_class "$URL_PATH" 2>&1 | tee -a output.txt
        RESULT=${PIPESTATUS[0]}
        ;;
    ADD_OBJECT_METADATA)
        printf '%s\n' \
        "---------------------------------------" \
        "Add the following metadata key=value pairs to $URL_PATH" \
        "$KEYVALUE_PAIRS" \
        "---------------------------------------" 2>&1 | tee -a output.txt
        add_metadata "$URL_PATH" "$KEYVALUE_PAIRS"  2>&1 | tee -a output.txt
        RESULT=${PIPESTATUS[0]}
        if [ "$RESULT" -ne 1 ]; then
            printf 'Result:\n'  2>&1 | tee -a output.txt
            list_metadata "$URL_PATH" 2>&1 | tee -a output.txt
        fi
        ;;
    REMOVE_OBJECT_METADATA)
        if [[ "$REMOVE_ALL" == "true" ]]; then
            printf '%s\n' \
            "---------------------------------------" \
            "Remove all metadata from $URL_PATH" \
            "---------------------------------------" 2>&1 | tee -a output.txt
            remove_all_metadata "$URL_PATH" 2>&1 | tee -a output.txt
            RESULT=${PIPESTATUS[0]}
        else
            printf '%s\n' \
            "---------------------------------------" \
            "Remove the following metadata keys from $URL_PATH" \
            "$KEYS" \
            "---------------------------------------" 2>&1 | tee -a output.txt
            remove_metadata "$URL_PATH" "$KEYS"  2>&1 | tee -a output.txt
            RESULT=${PIPESTATUS[0]}
        fi
        echo "---------------------------------------"  2>&1 | tee -a output.txt

        if [ "$RESULT" -ne 1 ]; then
            printf 'Result:\n'  2>&1 | tee -a output.txt
            list_metadata "$URL_PATH" 2>&1 | tee -a output.txt
        fi
        ;;
    FILTER_OBJECTS_BY_METADATA)
        printf '%s\n' \
        "---------------------------------------" \
        "List the objects in $BUCKET_PATH " \
        "which have ${FILTER_TYPE,,} set" \
        "---------------------------------------" 2>&1 | tee -a output.txt
        RESULT=0
        if [[ "$FILTER_TYPE" == "Specific Metadata" && -n "${KEYVALUE_PAIRS// }" ]]; then
            printf '%s\n' \
            "$KEYVALUE_PAIRS" \
            "(Note: only the key/value pairs for the specified metadata keys will be output)" \
            "---------------------------------------" 2>&1 | tee -a output.txt
            LIST_METADATA_WITH_OBJECT="true"
            list_objects_filtered_metadata "$BUCKET_PATH" "$KEYVALUE_PAIRS" 2>&1 | tee -a output.txt
            RESULT=${PIPESTATUS[0]}
        fi

        if [[ "$FILTER_TYPE" == "Any Metadata" || "$FILTER_TYPE" == "No Metadata" ]]; then
            expect_metadata=false
            if [[ "$FILTER_TYPE" == "Any Metadata" ]]; then
                expect_metadata=true
            fi
            list_objects_custom_metadata_status "$BUCKET_PATH" "$expect_metadata" 2>&1 | tee -a output.txt
            RESULT=${PIPESTATUS[0]}
        fi
        ;;
    UPDATE_METADATA_ON_FILTERED_OBJECTS)
        printf '%s\n' \
        "---------------------------------------" \
        "Select Objects in $BUCKET_PATH " \
        "which have custom metadata: $KEY_OR_KEYVALUE_PAIR" \
        "and change the value of that key to $UPDATE_VALUE" \
        "---------------------------------------" 2>&1 | tee -a output.txt
        update_metadata_filtered_objects "$BUCKET_PATH" "$KEY_OR_KEYVALUE_PAIR" "$UPDATE_VALUE"  2>&1 | tee -a output.txt
        RESULT=${PIPESTATUS[0]}
        ;;
    REMOVE_METADATA_ON_FILTERED_OBJECTS)
        printf '%s\n' \
        "---------------------------------------" \
        "Select Objects in $BUCKET_PATH " \
        "which have custom metadata: $KEY_OR_KEYVALUE_PAIR" \
        "and remove that metadata item" \
        "---------------------------------------" 2>&1 | tee -a output.txt
        remove_metadata_filtered_objects "$BUCKET_PATH" "$KEY_OR_KEYVALUE_PAIR"  2>&1 | tee -a output.txt
        RESULT=${PIPESTATUS[0]}
        ;;
    MOVE_FILTERED_OBJECTS)
        printf '%s\n' \
        "---------------------------------------" \
        "Change the storage class to $STORAGE_CLASS for the objects in $BUCKET_PATH " \
        "which have ${FILTER_TYPE,,} set" \
        "---------------------------------------" 2>&1 | tee -a output.txt
        if [[ "$FILTER_TYPE" == "Specific Metadata" && -n "${KEYVALUE_PAIRS// }" ]]; then
            if [[ -n "${KEYVALUE_PAIRS// }"  ]]; then
                printf '%s\n' \
                "$KEYVALUE_PAIRS" \
                "---------------------------------------" 2>&1 | tee -a output.txt
            fi
            ob_list=$(list_objects_filtered_metadata "$BUCKET_PATH" "$KEYVALUE_PAIRS" 2>&1 | tee -a output.txt)
        fi

        if [[ "$FILTER_TYPE" == "Any Metadata" || "$FILTER_TYPE" == "No Metadata" ]]; then
            expect_metadata=false
            if [[ "$FILTER_TYPE" == "Any Metadata" ]]; then
                expect_metadata=true
            fi
            ob_list=$(list_objects_custom_metadata_status "$BUCKET_PATH" "$expect_metadata" 2>&1 | tee -a output.txt)
        fi
        RESULT=0
        if [ -n "$ob_list" ]; then
            objectlist_move_storage "$ob_list" "$STORAGE_CLASS" 2>&1 | tee -a output.txt
            RESULT=${PIPESTATUS[0]}
        fi
        ;;
    DELETE_FILTERED_OBJECTS)
        printf '%s\n' \
        "---------------------------------------" \
        "Delete the objects in $BUCKET_PATH " \
        "which have ${FILTER_TYPE,,} set" \
        "---------------------------------------" 2>&1 | tee -a output.txt
        if [[ "$FILTER_TYPE" == "Specific Metadata" && -n "${KEYVALUE_PAIRS// }" ]]; then
            if [[ -n "${KEYVALUE_PAIRS// }"  ]]; then
                printf '%s\n' \
                "$KEYVALUE_PAIRS" \
                "---------------------------------------" 2>&1 | tee -a output.txt
            fi
            ob_list=$(list_objects_filtered_metadata "$BUCKET_PATH" "$KEYVALUE_PAIRS" 2>&1 | tee -a output.txt)
        fi

        if [[ "$FILTER_TYPE" == "Any Metadata" || "$FILTER_TYPE" == "No Metadata" ]]; then
            expect_metadata=false
            if [[ "$FILTER_TYPE" == "Any Metadata" ]]; then
                expect_metadata=true
            fi
            ob_list=$(list_objects_custom_metadata_status "$BUCKET_PATH" "$expect_metadata" 2>&1 | tee -a output.txt)
        fi
        RESULT=0
        if [ -n "$ob_list" ]; then
            if [[ "$DRY_RUN_ONLY" == "true" ]]; then
                printf 'DRY RUN ONLY - in a real run, the objects would be deleted\n' 2>&1 | tee -a output.txt
            else
                objectlist_delete_all "$ob_list"  2>&1 | tee -a output.txt
                RESULT=${PIPESTATUS[0]}
            fi
        fi
        ;;
    UPLOAD_OBJECT)
        echo "---------------------------------------"  2>&1 | tee -a output.txt

        if [[ -z "${KEYVALUE_PAIRS// }" ]]; then
            metadata_desc=""
        else
            metadata_desc="with metadata '$KEYVALUE_PAIRS'"
        fi
        echo "Upload the following item to $BUCKET_PATH $metadata_desc: $FILE"  2>&1 | tee -a output.txt

        object_upload "$FILE" "$BUCKET_PATH" "$KEYVALUE_PAIRS"
        RESULT=$?
        ;;
    *)
        # default
        ;;
esac
exit "${RESULT}"

