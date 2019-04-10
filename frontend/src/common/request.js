import * as Sentry from '@sentry/browser';

const rootPath = '/graphql';

const errors = {
  'InvalidJson': 'Invalid data format'
};

const getErrorText = (error) => {
  if (error.code && errors[error.code]) {
    const message = errors[error.code];
    if (typeof message === "string") {
      return message;
    }

    return message(error.data);
  }
  return 'Unkown error occurred';
};

const handleResponse = (response) => {
  if (!response.ok) {
    if (response.status >= 500) {
      Sentry.captureMessage(`Request to ${response.url} failed: ${response.status}`);
      return Promise.reject('Internal Server Error');
    } else if (response.status === 400 || response.status == 409) {
      // Bad Request or Conflict.
      return response.json().then(error => {
        return Promise.reject(getErrorText(error));
      });
    } else if (response.status === 401) {
      // Unauthorized.
      document.dispatchEvent(new Event('logout'));
      return Promise.reject('Unauthorized');
    }

    return Promise.reject(response.statusText);
  }
  return response.text();
};

const handleJSONResponse = (response) => {
  return handleResponse(response).then(content => Promise.resolve(JSON.parse(content)));
};

const request = (resource, init = {}, options = {}) => {

  const config = {
    auth: true,
    ...options
  };
  const headers = {
    ...init.headers
  };
  // if (config.auth) {
  //   headers.Authorization = `Bearer ${getToken()}`;
  // }
  return fetch(`${rootPath}/${resource}`.replace(/\/$/, ''), {
    ...init,
    headers: new Headers({ ...headers })
  }).then(config.responseType === 'json' ? handleJSONResponse : handleResponse);
};

export default request;
