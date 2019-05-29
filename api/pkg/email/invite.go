package email

const inviteTextTmpl = `
{{ .User }} has shared his Okteto space with you.

Okteto helps you accelerate the development of cloud native applications on Kubernetes and collaborate with your teammates. Join {{ .User }} to start developing directly in the cloud!

Please go to {{ .URL }} to join.

Cheers,
The Okteto team
`
