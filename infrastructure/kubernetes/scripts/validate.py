"""Render all resources and validate CNPG/plugin fields against pinned CRDs.

Usage: python validate.py /path/to/cnpg-1.29.2.yaml /path/to/barman-0.15.0.yaml
Use kubectl client/server dry-runs too: this does not replace admission checks.
"""

import copy
import subprocess
import sys
from pathlib import Path

import jsonschema
import yaml

ROOT = Path(__file__).resolve().parents[1]
OVERLAYS = [
    "apps/overlays/single-node", "apps/overlays/ha",
    "database/overlays/single-node", "database/overlays/ha",
    "security", "migrations", "maintenance/bootstrap",
    "maintenance/analytics-refresh", "maintenance/models-refresh",
    "addons/traefik/base", "addons/traefik/ha",
]


def strict(schema):
    result = copy.deepcopy(schema)
    if isinstance(result, dict):
        if result.get("type") == "object" and "properties" in result:
            if not result.get("x-kubernetes-preserve-unknown-fields"):
                result.setdefault("additionalProperties", False)
        for key, value in list(result.items()):
            result[key] = strict(value)
    elif isinstance(result, list):
        result = [strict(value) for value in result]
    return result


def main():
    schemas = {}
    for filename in sys.argv[1:]:
        for resource in yaml.safe_load_all(Path(filename).read_text()):
            if not resource or resource.get("kind") != "CustomResourceDefinition":
                continue
            spec = resource["spec"]
            for version in spec["versions"]:
                schemas[(spec["group"] + "/" + version["name"], spec["names"]["kind"])] = (
                    version["schema"]["openAPIV3Schema"]
                )
    if not schemas:
        raise SystemExit("Supply the pinned CNPG and Barman release manifests")
    for overlay in OVERLAYS:
        output = subprocess.check_output(["kubectl", "kustomize", str(ROOT / overlay)], text=True)
        resources = list(yaml.safe_load_all(output))
        for resource in resources:
            spec = resource.get("spec", {})
            kind = resource["kind"]
            if kind in {"Cluster", "ObjectStore", "ScheduledBackup"}:
                schema = schemas[(resource["apiVersion"], kind)]["properties"]["spec"]
                jsonschema.validate(spec, strict(schema))
            if kind == "Cluster":
                assert spec["instances"] == (3 if overlay.endswith("/ha") else 1)
                assert "@sha256:" in spec["imageName"]
                assert "barmanObjectStore" not in spec.get("backup", {})
            if kind == "Service":
                assert spec["type"] == "ClusterIP"
            if kind == "ConfigMap":
                assert resource["data"]["MODEL_REFRESH_POLICY"] == "manual"
                assert resource["data"]["CACHE_ENABLED"] == "false"
            if kind == "HelmChartConfig":
                values = yaml.safe_load(spec["valuesContent"])
                assert values["deployment"]["replicas"] == (2 if overlay.endswith("/ha") else 1)
                assert values["resources"]["limits"]
            if kind in {"Deployment", "Job"}:
                pod = spec["template"]["spec"]
                assert pod["securityContext"]["runAsNonRoot"]
                assert not pod["automountServiceAccountToken"]
                for container in pod["containers"]:
                    assert container["resources"]["requests"]
                    assert container["resources"]["limits"]
                    assert ":latest" not in container["image"]
                if resource["metadata"].get("name") == "pipeline-worker":
                    assert spec["replicas"] == 1
                    assert spec["strategy"]["type"] == "Recreate"
                    assert "livenessProbe" not in pod["containers"][0]
        print(f"PASS {overlay}: {len(resources)} resources")


if __name__ == "__main__":
    main()
