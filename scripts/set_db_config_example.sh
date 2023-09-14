vault write database/config/couchbasecapella-database \
    plugin_name="couchbasecapella-database-plugin" \
    cloud_api_base_url="https://cloudapi.cloud.couchbase.com/v4" \
    organization_id="cfc96f13-50e2-4746-b9a3-ad1bdb94f284" \
    project_id="e8f86798-2df4-4d75-b3d4-cbd64d8618df" \
    cluster_id="d1a9d1e3-80c7-4c2e-a6c5-e397f8341e6e" \
    username="llVo8WJ5gB6M5zycAvm1qYwOvo7hLwGF" \
    password='ZArg8AaHwUGY07caQ#2pwZGbEV#@kpX@x8C123Zu6BF!tSWibFjvuUQmFF4fXSnD' \
    password_policy="couchbasecapella" \
    allowed_roles="*" \
    username_template='{{printf "VC_%s_%s_%s_%s" (printf "%s" .DisplayName | uppercase | truncate 64) (printf "%s" .RoleName | uppercase | truncate 64) (random 20 | uppercase) (unix_time) | truncate 128}}'

