vault write database/config/couchbasecapella-database \
    plugin_name="couchbasecapella-database-plugin" \
    cloud_api_base_url="https://cloudapi.dev.nonprod-project-avengers.com/v4" \
    organization_id="6af08c0a-8cab-4c1c-b257-b521575c16d0" \
    project_id="d352361d-8de1-445b-9969-873b6decb63a" \
    cluster_id="1b40279a-6c27-44f1-bc7b-8d216563978e" \
    username="B749cMAAYxxJLU8BnD31J4WXoOGq4TW4" \
    password='sUZcQyfL4X61IeS#JR7mObOzXWrb8RoH%80VFf%ITx%yXH2EONNkUplwEc0zYBqC' \
    password_policy="couchbasecapella" \
    allowed_roles="*" \
    username_template='{{printf "VC_%s_%s_%s_%s" (printf "%s" .DisplayName | uppercase | truncate 64) (printf "%s" .RoleName | uppercase | truncate 64) (random 20 | uppercase) (unix_time) | truncate 128}}'

