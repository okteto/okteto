import { getToken } from 'common/environment';

const rootPath = '/graphql';

const errors = {
  'not-authorized': 'Session expired'
};

const getErrorText = (message) => {
  return errors[message] || 'Unknown error occurred';
};

const handleQLResponse = (response) => {
  return response.json().then(content => {
    if (content.errors && content.errors.length > 0) {
      for (let error of Array.from(content.errors)) {
        if (error.message === 'not-authorized') {
          document.dispatchEvent(new Event('logout'));
          return Promise.reject(getErrorText(error.message));
        }
      }
      return Promise.reject('Unknown error occurred');
    }
    return content;
  });
};

const request = (query = '', init = {}, options = {}) => {
  const config = {
    auth: true,
    ...options
  };

  const headers = {
    ...init.headers
  };

  if (config.auth) {
    headers.Authorization = `Bearer ${getToken()}`;
  }

  return fetch(`${rootPath}/`.replace(/\/$/, ''), {
    ...init,
    method: 'post',
    headers: new Headers({ ...headers }),
    body: JSON.stringify({ query })
  }).then(handleQLResponse);
};

export default request;
