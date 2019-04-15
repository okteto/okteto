import analytics from 'common/analytics';
import request from 'common/request';
import { notify } from 'components/Notification';

const fetchDatabases = () => {
  return request(`{ databases { name,endpoint } }`, { 
    auth: true
  });
};

export const requestDatabases = () => {
  return {
    type: 'REQUEST_DATABASES'
  };
};

export const receiveDatabases = databases => {
  return {
    type: 'RECEIVE_DATABASES',
    databases
  };
};

export const handleFetchError = err => {
  notify(`Error: ${err}`, 'error');
  return {
    type: 'HANDLE_FETCH_ERROR'
  };
};

export const refreshDatabases = () => {
  return dispatch => {
    dispatch(requestDatabases());
    fetchDatabases().then(e => {
      dispatch(receiveDatabases(e.data.databases));
    }).catch(err => dispatch(handleFetchError(err)));
  };
};

export const createDatabase = (name) => {
  return dispatch => {
    analytics.track('Create Database');

    return request(`mutation {
      createDatabase(name: "${name}") {
        name
      }
    }`, {
      auth: true
    }).then(() => {
      dispatch(refreshDatabases());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};

export const deleteDatabase = database => {
  return dispatch => {
    analytics.track('Delete Database');

    return request(`mutation {
      deleteDatabase(name: "${database.name}") {
        name
      }
    }`, {
      auth: true
    }).then(() => {
      dispatch(refreshDatabases());
    }).catch(err => notify(`Error: ${err}`, 'error'));
  };
};
