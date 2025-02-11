# terraform-manage-script-AWS
Go script that is used to manage terraform actions

## Notes

- Allows to the following:

    - pull tfvars
    - upload tfvars
    - terraform plans
    - terraform applies

- There are not credentials stored in the script that makes it easy to use in secure enviornments and in pipelines where things will be set using envs
