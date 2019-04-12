import request from 'common/request';
import { notify } from 'components/Notification';
import environment from 'common/environment';

export const SESSION_KEY = 'okteto-session';

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
      if (e.errors) {
        notify(`Authentication error: ${e.errors[0].message}`, 'error')
      } else {
        localStorage.setItem(environment.apiTokenKeyName, e.data.auth.token);
        dispatch(authSuccess(e.data.auth));
        dispatch(saveSession());
      }
    }).catch(err => notify(`Authentication error: ${err}`, 'error'));
  };
};

export const authSuccess = user => {
  return {
    type: 'AUTH_SUCCESS',
    user
  };
};

export const logout = () => {
  localStorage.removeItem(SESSION_KEY);
  return { type: 'LOGOUT' };
};

export const restoreSession = () => {
  return (dispatch) => {
    const session = JSON.parse(localStorage.getItem(SESSION_KEY)) || {};
    
    dispatch(restoreSessionSuccess(session));

    if (session.user && session.user.id) {
      // mixpanel.identify(session.user.userID, {
      //   origin: 'Restored Session'
      // });
    }
  };
};

export const restoreSessionSuccess = (session) => {
  return { 
    type: 'RESTORE_SESSION_SUCCESS',
    session
  };
};

export const saveSession = () => {
  return { type: 'SAVE_SESSION' };
};

export const updateSession = user => {
  return { 
    type: 'UPDATE_SESSION',
    user
  };
};
