import { getToken } from 'common/environment';
import { GraphQLClient } from 'graphql-request';

const gqlEndpoint = '/graphql';

const errors = {
  'not-authorized': 'Session expired'
};

const getErrorText = message => {
  return errors[message] || message;
};

const handleGqlErrors = error => {
  if (error.response) { // GraphQL error response.
    const errors = error.response.errors;
    if (errors && errors.length > 0) {
      for (let error of Array.from(errors)) {
        if (error.message === 'not-authorized') {
          document.dispatchEvent(new Event('logout'));
        }
        return Promise.reject(getErrorText(error.message));
      }
    }
  } else {
    return Promise.reject(navigator.onLine ? 
      'Server is not available.' : 
      'Check your Internet connection.'
    );
  }
};

const request = async (query = '', variables = {}, options = { auth: true }) => {
  const graphQLClient = new GraphQLClient(gqlEndpoint, {
    headers: options.auth ? { authorization: `Bearer ${getToken()}` } : {}
  })

  try {
    const data = await graphQLClient.request(query, variables);
    return data;
  } catch (error) {
    return handleGqlErrors(error);
  }
};

export default request;
