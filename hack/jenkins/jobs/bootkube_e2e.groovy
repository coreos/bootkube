// META
job_dir = 'bootkube'

// CONFIG
fork_to_use = 'kubernetes-incubator'
job_admins = ['colemickens', 'ericchiang', 'rithujohn191', 'rphillips']
user_whitelist = job_admins
org_whitelist = ['coreos', 'coreos-inc']

// JOBS
network_providers = ['flannel', 'calico']
network_providers.each { np ->
  // Note: the "tku-" prefix is to differentiate "team-kube-upstream" jenkins job
  // statuses and triggers from the legacy jobs and triggers.
  job_name = "tku-bootkube-e2e-${np}"

  pipelineJob(job_name) {
    parameters {
      stringParam('sha1', 'origin/master', 'git reference to build')
    }
    definition {
      triggers {
        githubPullRequest {
          admins(job_admins)
          userWhitelist(user_whitelist)
          orgWhitelist(org_whitelist)
          useGitHubHooks(true)
          onlyTriggerPhrase(false)
          triggerPhrase("coreosbot run ${job_name}")

          extensions {
            commitStatus {
              // TODO: this is dependent on this merging, or using my fork: https://github.com/awslabs/aws-js-s3-explorer/pull/16
              statusUrl('https://bootkube-pr-logs.s3-us-west-2.amazonaws.com/index.html#pr/${JOB_NAME}-${BUILD_NUMBER}/')
              context(job_name)
              triggeredStatus('e2e triggered')
              startedStatus('e2e started')
              completedStatus('SUCCESS', 'e2e succeeded')
              completedStatus('FAILURE', 'e2e failed. Investigate!')
              completedStatus('PENDING', 'e2e queued')
              completedStatus('ERROR', 'e2e internal error. Investigate!')
            }
          }
        }
      }

      cpsScm {
        scm {
          git {
            remote {
              github("${fork_to_use}/bootkube")
              refspec('+refs/pull/*:refs/remotes/origin/pr/*')
              credentials('github_userpass')
            }
            branch('${sha1}')
          }
        }
        scriptPath('hack/jenkins/pipelines/bootkube-e2e/Jenkinsfile')
      }
    }
  }
}
