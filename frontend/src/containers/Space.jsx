import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import * as clipboard from 'clipboard-polyfill';
import autobind from 'autobind-decorator';

import { refreshEnvironments } from 'actions/environments';
import { notify } from 'components/Notification';
import Button from 'components/Button';
import Hint from 'components/Hint';
import Icon from 'components/Icon';
import Header from './Header';
import DeleteDialog from './DeleteDialog';

import 'containers/Space.scss';

const POLLING_INTERVAL = 10000;

class Space extends Component {
  constructor(props) {
    super(props);

    this.state = {
      showNewHint: false
    };

    this.props.dispatch(refreshEnvironments());
    this.poll = setInterval(this.handlePollEnvironments, POLLING_INTERVAL);
  }

  componentWillUnmount() {
    clearInterval(this.poll);
  }

  @autobind
  handlePollEnvironments() {
    this.props.dispatch(refreshEnvironments());
  }

  @autobind
  handleDelete(environment) {
    this.deleteDialog.getWrappedInstance().open(environment);
  }

  render() {
    const { environments, user } = this.props;
    const environmentList = Object.values(environments);

    const HintContent = () => (
      <div className="layout vertical">
        <h1>Start a new environment</h1>
        
        <div className="step layout vertical">
          <p>
            <span className="number">1.</span> Install <strong>Okteto CLI</strong>:
          </p>
          <div className="layout horizontal">
            <code className="cli flex-auto">
              curl https://get.okteto.com -sSfL | sh
            </code>
            <Button
              className="ClipboardButton"
              icon="clipboard"
              iconSize="24"
              onClick={() => {
                clipboard.writeText(`curl https://get.okteto.com -sSfL | sh`);
                notify('Copied to clipboard!');
                // mixpanel.track('Copied CLI Command');
              }}
              light
              frameless>
            </Button>
          </div>
        </div>

        <div className="step layout vertical">
          <p>
            <span className="number">2.</span> From your <strong>local repository</strong> launch 
            okteto:
          </p>
          <div className="layout horizontal">
            <code className="cli flex-auto">
              okteto up
            </code>
            <Button
              className="ClipboardButton"
              icon="clipboard"
              iconSize="24"
              onClick={() => {
                clipboard.writeText(`okteto up`);
                notify('Copied to clipboard!');
                // mixpanel.track('Copied CLI Command');
                this.setState({ showNewHint: false });
              }}
              light
              frameless>
            </Button>
          </div>
        </div>

        <div className="step layout vertical">
          <p>
            <span className="number">3.</span> Code in the cluster from your favorite IDE!
          </p>
          <div className="layout horizontal">
            <code className="cli cli-okteto flex-auto">
              yarn start<br/> Running server at http://{user.id}.okteto.net
            </code>
          </div>
        </div>

      </div>
    );

    return (
      <div className="Space layout vertical">
        <Header title={`${user.githubID}'s space`} />

        {environmentList.length === 0 &&
          <div className="EmptySpace layout vertical center">
            <Icon icon="emptySpace" size="140" />
            <h2>Your space is empty.</h2>
            <div style={{
              position: 'relative'
            }}>
              <Hint 
                className="HintNew"
                open={true}
                width="532"
                arrowPosition="center"
                offsetY="24"
                offsetX="0"
                positionX="center"
                positionY="bottom"
                hideCloseButton
              >
                <HintContent />
              </Hint>
            </div>
          </div>
        }

        {environmentList.length > 0 && 
          <>
            <div className="EnvironmentList layout vertical">
              {environmentList.map(environment =>
                <div key={environment.id} className="EnvironmentItem layout horizontal start">
                  <div className="EnvironmentItemIcon">
                    <Icon icon="mirror" size="20"/>
                  </div>
                  <div className="EnvironmentItemName ellipsis" 
                    title={environment.name}>
                    {environment.name}
                  </div>
                  <div className="EnvironmentItemEndpoints layout vertical">
                    {environment.endpoints.map(url =>
                      <a className="ellipsis layout horizontal center" 
                        key={`${environment.id}-${url}`}
                        href={url}
                        rel="noreferrer noopener" 
                        target="_blank">
                        {url}
                        <Icon icon="external" size="18" />
                      </a>
                    )}
                  </div>
                  <div className="flex-auto" />
                  <div className="EnvironmentItemActions layout horizontal center">
                    <div className="ActionButton" onClick={() => this.handleDelete(environment)}>
                      <Icon icon="delete" size="24" />
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="ActionBar layout horizontal center">
              <div className="flex-auto"></div>
              <div className="NewButtonContainer">
                <Button 
                  className="NewButton" 
                  icon="plusCircle" 
                  iconSize="18"
                  onClick={() => this.setState({ showNewHint: true })}
                >
                  New Environment
                </Button>
                <Hint 
                  className="HintNew"
                  open={this.state.showNewHint}
                  onClose={() => this.setState({ showNewHint: false })}
                  width="520"
                  arrowPosition="right"
                  offsetY="46"
                  offsetX="-50"
                  positionX="left"
                >
                  <HintContent />
                </Hint>
              </div>
            </div>
          </>
        }

        <DeleteDialog ref={ref => this.deleteDialog = ref} />
      </div>
    );  
  }
}

Space.defaultProps = {
};

Space.propTypes = {
  dispatch: PropTypes.func,
  user: PropTypes.object.isRequired,
  environments: PropTypes.object.isRequired
};

export default ReactRedux.connect(state => {
  return {
    environments: state.environments.byId || {},
    user: state.session.user
  };
})(Space);