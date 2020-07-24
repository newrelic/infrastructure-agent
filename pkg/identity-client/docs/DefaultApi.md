# \DefaultApi

All URIs are relative to *https://localhost:443/identity/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ConnectPost**](DefaultApi.md#ConnectPost) | **Post** /connect | Connect using a fingerprint
[**ConnectPut**](DefaultApi.md#ConnectPut) | **Put** /connect | Reconnect using a fingerprint and an entityID
[**DisconnectPut**](DefaultApi.md#DisconnectPut) | **Put** /disconnect | disconnects an entity
[**LookupBatchPost**](DefaultApi.md#LookupBatchPost) | **Post** /lookup/batch | lookup batch for list of entities, given their entityNames
[**LookupPost**](DefaultApi.md#LookupPost) | **Post** /lookup | lookup for an entity, given the entityName
[**RegisterBatchPost**](DefaultApi.md#RegisterBatchPost) | **Post** /register/batch | Register integration entities in batch of a max size of 1000 and 1MB
[**RegisterBatchPut**](DefaultApi.md#RegisterBatchPut) | **Put** /register/batch | Re-register integration entities in batch of a max size of 1000 and 1MB
[**RegisterPost**](DefaultApi.md#RegisterPost) | **Post** /register | Register integration entity
[**RegisterPut**](DefaultApi.md#RegisterPut) | **Put** /register | Re-register integration entity



## ConnectPost

> ConnectResponse ConnectPost(ctx, userAgent, xLicenseKey, connectRequest, optional)

Connect using a fingerprint

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**userAgent** | **string**|  | 
**xLicenseKey** | **string**|  | 
**connectRequest** | [**ConnectRequest**](ConnectRequest.md)|  | 
 **optional** | ***ConnectPostOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ConnectPostOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **contentEncoding** | **optional.String**|  | 

### Return type

[**ConnectResponse**](ConnectResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ConnectPut

> ConnectResponse ConnectPut(ctx, userAgent, xLicenseKey, reconnectRequest, optional)

Reconnect using a fingerprint and an entityID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**userAgent** | **string**|  | 
**xLicenseKey** | **string**|  | 
**reconnectRequest** | [**ReconnectRequest**](ReconnectRequest.md)|  | 
 **optional** | ***ConnectPutOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ConnectPutOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **contentEncoding** | **optional.String**|  | 

### Return type

[**ConnectResponse**](ConnectResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DisconnectPut

> DisconnectPut(ctx, userAgent, xLicenseKey, disconnectRequest, optional)

disconnects an entity

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**userAgent** | **string**|  | 
**xLicenseKey** | **string**|  | 
**disconnectRequest** | [**DisconnectRequest**](DisconnectRequest.md)|  | 
 **optional** | ***DisconnectPutOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DisconnectPutOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **contentEncoding** | **optional.String**|  | 

### Return type

 (empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: Not defined

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## LookupBatchPost

> []map[string]interface{} LookupBatchPost(ctx, userAgent, xLicenseKey, lookupRequest, optional)

lookup batch for list of entities, given their entityNames

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**userAgent** | **string**|  | 
**xLicenseKey** | **string**|  | 
**lookupRequest** | [**[]LookupRequest**](LookupRequest.md)|  | 
 **optional** | ***LookupBatchPostOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a LookupBatchPostOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **contentEncoding** | **optional.String**|  | 

### Return type

[**[]map[string]interface{}**](map[string]interface{}.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json, text/plain

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## LookupPost

> LookupResponse LookupPost(ctx, userAgent, xLicenseKey, lookupRequest, optional)

lookup for an entity, given the entityName

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**userAgent** | **string**|  | 
**xLicenseKey** | **string**|  | 
**lookupRequest** | [**LookupRequest**](LookupRequest.md)|  | 
 **optional** | ***LookupPostOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a LookupPostOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **contentEncoding** | **optional.String**|  | 

### Return type

[**LookupResponse**](LookupResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json, text/plain

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## RegisterBatchPost

> []RegisterBatchEntityResponse RegisterBatchPost(ctx, userAgent, xLicenseKey, registerRequest, optional)

Register integration entities in batch of a max size of 1000 and 1MB

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**userAgent** | **string**|  | 
**xLicenseKey** | **string**|  | 
**registerRequest** | [**[]RegisterRequest**](RegisterRequest.md)|  | 
 **optional** | ***RegisterBatchPostOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a RegisterBatchPostOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **contentEncoding** | **optional.String**|  | 
 **xNRIAgentEntityId** | **optional.Int64**|  | 

### Return type

[**[]RegisterBatchEntityResponse**](RegisterBatchEntityResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json, text/plain

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## RegisterBatchPut

> []ReRegisterBatchEntityResponse RegisterBatchPut(ctx, userAgent, xLicenseKey, reRegisterRequest, optional)

Re-register integration entities in batch of a max size of 1000 and 1MB

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**userAgent** | **string**|  | 
**xLicenseKey** | **string**|  | 
**reRegisterRequest** | [**[]ReRegisterRequest**](ReRegisterRequest.md)|  | 
 **optional** | ***RegisterBatchPutOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a RegisterBatchPutOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **contentEncoding** | **optional.String**|  | 
 **xNRIAgentEntityId** | **optional.Int64**|  | 

### Return type

[**[]ReRegisterBatchEntityResponse**](ReRegisterBatchEntityResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json, text/plain

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## RegisterPost

> RegisterResponse RegisterPost(ctx, userAgent, xLicenseKey, registerRequest, optional)

Register integration entity

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**userAgent** | **string**|  | 
**xLicenseKey** | **string**|  | 
**registerRequest** | [**RegisterRequest**](RegisterRequest.md)|  | 
 **optional** | ***RegisterPostOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a RegisterPostOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **contentEncoding** | **optional.String**|  | 
 **xNRIAgentEntityId** | **optional.Int64**|  | 

### Return type

[**RegisterResponse**](RegisterResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json, text/plain

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## RegisterPut

> ReRegisterResponse RegisterPut(ctx, userAgent, xLicenseKey, reRegisterRequest, optional)

Re-register integration entity

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**userAgent** | **string**|  | 
**xLicenseKey** | **string**|  | 
**reRegisterRequest** | [**ReRegisterRequest**](ReRegisterRequest.md)|  | 
 **optional** | ***RegisterPutOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a RegisterPutOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **contentEncoding** | **optional.String**|  | 
 **xNRIAgentEntityId** | **optional.Int64**|  | 

### Return type

[**ReRegisterResponse**](ReRegisterResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json, text/plain

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

