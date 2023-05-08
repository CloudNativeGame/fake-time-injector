#!/bin/bash

#This script cleans the certs secret generated when installed and the mutation web hook configuration

kubectl -n kube-system delete secret kubernetes-webhook-injector-certs
kubectl delete mutatingwebhookconfigurations.admissionregistration.k8s.io kubernetes-webhook-injector