#!/usr/bin/env bash

# doc
# https://docs.newrelic.com/docs/alerts-applied-intelligence/new-relic-alerts/advanced-alerts/rest-api-alerts/rest-api-calls-alerts/#alert-policies

##########################################
# Alert policy
##########################################
curl -X POST 'https://staging-api.newrelic.com/v2/alerts_policies.json' \
    -H 'Api-Key:NR_API_KEY' -i \
    -H 'Content-Type: application/json' \
    -d \
'{
 "policy": {
   "incident_preference": "PER_POLICY",
   "name": "Cpu percent - Centos8/Redhat7 Test"
 }
}'

 {"policy":{"name":"gmerkushev test 1","incident_preference":"PER_POLICY"}}

Response:
{
 "policy": {
   "id": 194913,
   "incident_preference": "PER_POLICY",
   "name": "Cpu percent - Centos8/Redhat7 Test",
   "created_at": 1634302571179,
   "updated_at": 1634302571179
 }
}

##########################################
# Create NRQL condition
##########################################

read -r -d '' NRQL_ALERT << EOM
{
 "nrql_condition":
   {
     "type": "static",
     "name": "Cpu percent - centos-stream/Redhat8 Test",
     "enabled": true,
     "value_function": "single_value",
     "violation_time_limit_seconds": 259200,
     "terms": [
       {
         "duration": "5",
         "operator": "above",
         "threshold": "3.0",
         "time_function": "all",
         "priority": "critical"
       }
     ],
     "nrql": {
       "query": "SELECT abs(filter(average(cpuPercent),WHERE displayName like '%centos-stream%')  - filter(average(cpuPercent),WHERE displayName like '%redhat-8%'))  FROM SystemSample WHERE displayName LIKE '%centos-stream%' OR displayName  LIKE '%centos-8%'"
     },
     "signal": {
       "aggregation_window": "60",
       "aggregation_method": "EVENT_FLOW",
       "aggregation_delay": 120,
       "fill_option": "none"
     }
   }
}
EOM


curl -X POST 'https://staging-api.newrelic.com/v2/alerts_nrql_conditions/policies/195439.json' \
    -H 'Api-Key:NR_API_KEY' -i \
    -H 'Content-Type: application/json' \
    -d "$NRQL_ALERT"

{
  "nrql_condition": {
    "id": 684668,
    "type": "static",
    "name": "Cpu percent - centos-stream/Redhat8 Test",
    "enabled": true,
    "value_function": "single_value",
    "violation_time_limit_seconds": 259200,
    "terms": [
      {
        "duration": "5",
        "operator": "above",
        "threshold": "3.0",
        "time_function": "all",
        "priority": "critical"
      }
    ],
    "nrql": {
      "query": "SELECT abs(filter(average(cpuPercent),WHERE displayName like '%centos-stream%')  - filter(average(cpuPercent),WHERE displayName like '%redhat-8%'))  FROM SystemSample WHERE displayName LIKE '%centos-stream%' OR displayName  LIKE '%centos-8%'"
    },
    "signal": {
      "aggregation_window": "60",
      "aggregation_method": "EVENT_FLOW",
      "aggregation_delay": 120,
      "fill_option": "none"
    }
  }
}


#################################
# List Channels
#################################

curl -X GET 'https://staging-api.newrelic.com/v2/alerts_channels.json' \
    -H 'Api-Key:NR_API_KEY' -i \


{
 "channels": [
   {
     "id": 1676261,
     "name": "caos-team@newrelic.com",
     "type": "email",
     "configuration": {
       "recipients": "caos-team@newrelic.com"
     },
     "links": {
       "policy_ids": [
         194836,
         194620
       ]
     }
   },
   {
     "id": 1423613,
     "name": "Cristian Ciutea <cciutea@newrelic.com>",
     "type": "user",
     "configuration": {
       "user_id": "914979"
     },
     "links": {
       "policy_ids": []
     }
   },
   {
     "id": 1587863,
     "name": "David Gay i Tèllo <dgay@newrelic.com>",
     "type": "user",
     "configuration": {
       "user_id": "291384"
     },
     "links": {
       "policy_ids": []
     }
   },
   {
     "id": 1423614,
     "name": "Grigorii Merkushev <gmerkushev@newrelic.com>",
     "type": "user",
     "configuration": {
       "user_id": "1370635"
     },
     "links": {
       "policy_ids": []
     }
   },
   {
     "id": 1423612,
     "name": "Juan Hernandez <jhernandez@newrelic.com>",
     "type": "user",
     "configuration": {
       "user_id": "883022"
     },
     "links": {
       "policy_ids": []
     }
   },
   {
     "id": 1423520,
     "name": "Noelia Diez <ndiez@newrelic.com>",
     "type": "user",
     "configuration": {
       "user_id": "1234878"
     },
     "links": {
       "policy_ids": []
     }
   },
   {
     "id": 26364,
     "name": "Toni Reina Perez <treinaperez@newrelic.com>",
     "type": "user",
     "configuration": {
       "user_id": "776191"
     },
     "links": {
       "policy_ids": []
     }
   }
 ],
 "links": {
   "channel.policy_ids": "/v2/policies/{policy_id}"
 }
}


#################################
# Add channel to policy
#################################
curl -X PUT 'https://staging-api.newrelic.com/v2/alerts_policy_channels.json' \
    -H 'Api-Key:NR_API_KEY' -i \
    -H 'Content-Type: application/json' \
    -G -d 'policy_id=195439&channel_ids=1482699'

 policy_id=195543&channel_ids=1676261

response:
{
  "policy": {
    "id": 195439,
    "channel_ids": [
      1482699
    ]
  }
}

#################################
# Delete policy
#################################
curl -X DELETE 'https://staging-api.newrelic.com/v2/alerts_policies/195439.json' \
    -H 'Api-Key:NR_API_KEY' -i \

response:
{
  "policy": {
    "id": 195439,
    "incident_preference": "PER_POLICY",
    "name": "Cpu percent - Centos8/Redhat7 Test",
    "created_at": 1636566135672,
    "updated_at": 1636566135672
  }
}