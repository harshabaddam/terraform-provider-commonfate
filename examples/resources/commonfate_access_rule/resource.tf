resource "commonfate_access_rule" "aws-admin" {
  name        = "This was made in terraform demo"
  description = "Access rule made in terraform"
  groups      = ["common_fate_administrators"]
  approval = {
    users = [""]
  }

  target = [
    {
      field = "accountId"
      value = ["123456789012"]
    },
    {
      field = "permissionSetArn"
      value = ["arn:aws:sso:::permissionSet/ssoins-825968feece9a0b6/3hjdfkj3r28ef"]
    }
  ]
  target_provider_id = "aws-sso-v2"
  duration           = "3600"
}
