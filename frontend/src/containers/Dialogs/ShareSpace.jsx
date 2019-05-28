import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import SpaceInvite from 'containers/SpaceInvite';
import Button from 'components/Button';
import Modal from 'components/Modal';
import { shareSpace } from 'actions/spaces';

import './ShareSpace.scss';

class ShareSpace extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  handleConfirmClick() {
    const members = this.spaceInviteInput.getMembers();
    this.props.dispatch(shareSpace(this.props.space.id, members));
    this.close();
  }

  @autobind
  handleCancelClick() {
    this.close();
  }

  open() {
    this.dialog && this.dialog.open();
    this.spaceInviteInput.reset();
    this.spaceInviteInput.focus();
  }

  close() {
    this.dialog && this.dialog.close();
  }

  render() {
    const { space } = this.props;
    const members = space.members.map(member => {
      return {
        username: member.githubID,
        email: member.email,
        owner: member.owner
      }; 
    });
    
    return (
      <Modal
        className="ShareSpace"
        ref={ref => this.dialog = ref} 
        title="Share Space"
        width={450}>
        <div className="create-dialog-content layout vertical">
          <SpaceInvite
            members={members}
            ref={ref => this.spaceInviteInput = ref}
          />

          <div className="Buttons layout horizontal-reverse center">
            <Button
              color="green"
              solid
              onClick={this.handleConfirmClick}>
              Save
            </Button>
            <Button
              color="grey"
              solid
              secondary
              onClick={this.handleCancelClick}>
              Cancel
            </Button>
          </div>
        </div>
      </Modal>
    );
  }
}

ShareSpace.propTypes = {
  dispatch: PropTypes.func.isRequired,
  space: PropTypes.object.isRequired
};

export default ReactRedux.connect(() => {
  return {};
}, null, null, { withRef: true })(ShareSpace);