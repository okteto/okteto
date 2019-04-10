import request from 'common/request';
import { notify } from 'components/Notification';
import environment from 'common/environment';

export const loginWithGithub = code => {
  return dispatch => {
    return request(``, {
      method: 'post',
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify(
        { query: `mutation{ auth(code:"${code}"){ id,name,email,token } }` })
    }, {
      responseType: 'json'
    }).then(e => {
      localStorage.setItem(environment.apiTokenKeyName, e.data.auth.token);
      dispatch(authSuccess(e.data.auth));
    }).catch(err => notify(`Authentication error: ${err}`, 'error'));
  };
};

export const authSuccess = (user) => {
  return {
    type: 'AUTH_SUCCESS',
    user
  };
};

export const logout = () => {
  return { type: 'LOGOUT' };
};

export const afterRestoreSession = (session) => {
  return { 
    type: 'AFTER_RESTORE_SESSION',
    session: session
  };
};

export const saveSession = () => {
  return { type: 'SAVE_SESSION' };
};

export const updateSession = (user) => {
  return { 
    type: 'UPDATE_SESSION',
    user
  };
};

export const refreshSession = () => {
  return (dispatch) => {
    return request(`/users`, {
      method: 'get'
    }, {
      responseType: 'json'
    }).then(user => {
      dispatch(updateSession(user));
    }).catch(err => notify(`Session error: ${err}`, 'error'));
  };
};

export const deleteAccount = () => {
  return (dispatch) => {
    return request(`/users`, {
      method: 'delete',
      headers: {
        "Content-Type": "application/json"
      }
    }).then(() => {
      dispatch(logout());
    }).catch(err => notify(`Authentication error: ${err}`, 'error'));
  };
};