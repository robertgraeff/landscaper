#!/bin/bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# COMPONENT_DIR is the is the path to the directory that contains the component-descriptor.yaml and resources.yaml
COMPONENT_DIR=$1

# TRANSPORT_FILE is the path to the transport tar file that will be created and pushed to the oci registry
TRANSPORT_FILE=$2

echo "Component directory: ${COMPONENT_DIR}"
echo "Transport file:      ${TRANSPORT_FILE}"

if [ -f "${TRANSPORT_FILE}" ]; then
  echo "Removing old transport file"
  rm "${TRANSPORT_FILE}"
fi

echo "Creating transport file"
RESOURCES_FILE="${COMPONENT_DIR}/resources.yaml"
if [ -f "${RESOURCES_FILE}" ]; then
    landscaper-cli component-cli component-archive "${COMPONENT_DIR}" "${TRANSPORT_FILE}" -r ${RESOURCES_FILE}
else
    landscaper-cli component-cli component-archive "${COMPONENT_DIR}" "${TRANSPORT_FILE}"
fi

echo "Pushing transport file to oci registry"
component-cli ctf push "${TRANSPORT_FILE}"
