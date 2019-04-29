import { getToken } from 'common/environment';

const rootPath = '/graphql';
const API_ERROR = 'api_error';

const errors = {
  'not-authorized': 'Session expired'
};

const getErrorText = message => {
  return errors[message] || message;
};

const handleQLResponse = (response) => {
  return response.json()
    .then(content => {
      if (content.errors && content.errors.length > 0) {
        for (let error of Array.from(content.errors)) {
          if (error.message === 'not-authorized') {
            document.dispatchEvent(new Event('logout'));
          }
          return Promise.reject({
            type: API_ERROR,
            text: getErrorText(error.message)
          });
        }
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
  })
    .then(handleQLResponse)
    .catch(error => {
      if (!error.type) {
        return Promise.reject(navigator.onLine ? 
          'Server is not available.' : 
          'Check your Internet connection.'
        );
      } else {
        return Promise.reject(error.text);
      }
    });
};

export default request;
