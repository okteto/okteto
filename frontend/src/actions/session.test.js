import request from 'common/request';
import analytics from 'common/analytics';
import environment from 'common/environment';
import { notify } from 'components/Notification';
import configureMockStore from 'redux-mock-store';
import thunk from 'redux-thunk';
import * as actions from './session';

const middlewares = [ thunk ];
const mockStore = configureMockStore(middlewares);
const githubCode = 'githubCodeGivenAfterAuthFlow';

jest.mock('common/request');
jest.mock('common/analytics');
jest.mock('components/Notification');

const initialStore = {   
  user: {},
  isAuthenticated: false
};

const userData = {
  token: 'sessionGivenToken'
};

const authSuccessResponse = {
  auth: userData
};

describe('Session Login', () => {
  afterEach(() => {
    request.mockReset();
  });

  it('should set user data on successful auth', async () => {
    const store = mockStore(initialStore);
    const expectedToken = 'sessionGivenToken';
    request.mockResolvedValue(authSuccessResponse);

    await store.dispatch(actions.loginWithGithub(githubCode));

    expect(store.getActions()).toEqual([
      { type: 'AUTH_SUCCESS', user: { token: expectedToken } },
      { type: 'SAVE_SESSION' }
    ]);
    expect(localStorage.setItem).toHaveBeenLastCalledWith(
      environment.apiTokenKeyName, expectedToken
    );
  });

  it('should properly initialize analytics', async () => {
    const store = mockStore(initialStore);
    request.mockResolvedValue(authSuccessResponse);

    await store.dispatch(actions.loginWithGithub(githubCode));
    
    expect(analytics.init).toHaveBeenLastCalledWith(userData);
    expect(analytics.increment).toHaveBeenLastCalledWith('Logins');
  });

  it('should properly notify graphql errors', async () => {
    const store = mockStore(initialStore);
    request.mockResolvedValue({
      errors: [{ message: 'An error occurred' }] 
    });

    await store.dispatch(actions.loginWithGithub(githubCode));
    
    expect(notify).toHaveBeenLastCalledWith(
      `Authentication error: An error occurred`, 
      'error'
    );
  });

  it('should properly notify request errors', async () => {
    const store = mockStore(initialStore);
    request.mockRejectedValue('Session expired');

    await store.dispatch(actions.loginWithGithub());
    
    expect(notify).toHaveBeenLastCalledWith(
      `Authentication error: Session expired`, 
      'error'
    );
  });
});

describe('Session Logout', () => {
  it('should logout and restore session data', () => {
    const expectedAction = { 
      type: 'LOGOUT' 
    };
    expect(actions.logout()).toEqual(expectedAction);
    expect(localStorage.removeItem).toHaveBeenLastCalledWith(actions.SESSION_KEY);
  });
})